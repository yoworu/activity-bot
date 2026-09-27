package application

import (
	"activity-bot/internal/predicate"
	"activity-bot/internal/roles"
	"fmt"

	"github.com/gotd/botapi"
)

const genshinFandom = "Genshin Impact"

func (h *Handler) sendRegionPrompt(c *botapi.Context) error {
	markup, err := h.regionMarkup(c)
	if err != nil {
		return err
	}

	_, err = c.Reply(
		h.regionPrompt(),
		botapi.WithParseMode(botapi.ParseModeHTML),
		botapi.DisableWebPagePreview(),
		botapi.WithReplyMarkup(markup),
	)

	return err
}

func (h *Handler) sendRegionPromptTo(c *botapi.Context, chatID int64) error {
	markup, err := h.regionMarkup(c)
	if err != nil {
		return err
	}

	_, err = c.Bot.SendMessage(
		c,
		botapi.ID(chatID),
		h.regionPrompt(),
		botapi.WithParseMode(botapi.ParseModeHTML),
		botapi.DisableWebPagePreview(),
		botapi.WithReplyMarkup(markup),
	)

	return err
}

func (h *Handler) editRegionPrompt(c *botapi.Context, chatID int64, messageID int) error {
	markup, err := h.regionMarkup(c)
	if err != nil {
		return err
	}

	_, err = c.Bot.EditMessageText(
		c,
		botapi.ID(chatID),
		messageID,
		h.regionPrompt(),
		botapi.WithParseMode(botapi.ParseModeHTML),
		botapi.DisableWebPagePreview(),
		botapi.WithReplyMarkup(markup),
	)
	if err != nil {
		return fmt.Errorf("edit region keyboard: %w", err)
	}

	return nil
}

func (h *Handler) regionPrompt() string {
	return "Выберите регион"
}

func (h *Handler) regionMarkup(c *botapi.Context) (*botapi.InlineKeyboardMarkup, error) {
	fandom, err := h.loadGenshinFandom(c)
	if err != nil {
		return nil, err
	}

	availability, err := h.loadAvailability(c)
	if err != nil {
		return nil, err
	}

	visible := make([]roles.Category, 0, len(fandom.Categories))

	for _, category := range fandom.Categories {
		if len(availability.filter(category.Roles)) == 0 {
			continue
		}

		visible = append(visible, category)
	}

	return regionKeyboard(visible), nil
}

func (h *Handler) refreshRoleKeyboard(c *botapi.Context, categoryID int64) error {
	cq := c.Update.CallbackQuery
	if cq == nil || cq.Message == nil {
		return nil
	}

	fandom, err := h.loadGenshinFandom(c)
	if err != nil {
		return err
	}

	var category *roles.Category

	for i := range fandom.Categories {
		if fandom.Categories[i].ID == categoryID {
			category = &fandom.Categories[i]
			break
		}
	}

	if category == nil {
		return h.editRegionPrompt(c, cq.Message.Chat.ID, cq.Message.MessageID)
	}

	available, err := h.availableRoles(c, category.Roles)
	if err != nil {
		return err
	}

	text := "Выбрать роль"

	if len(available) == 0 {
		text = "В этом регионе нет свободных ролей"
	}

	_, err = c.Bot.EditMessageText(
		c,
		botapi.ID(cq.Message.Chat.ID),
		cq.Message.MessageID,
		text,
		botapi.WithReplyMarkup(roleKeyboard(available)),
	)
	if err != nil {
		return fmt.Errorf("refresh role keyboard: %w", err)
	}

	return nil
}

func (h *Handler) loadGenshinFandom(c *botapi.Context) (roles.Fandom, error) {
	fandom, err := h.rolesRepository.GetRoleTemplate(
		c,
		h.targetChatID,
		genshinFandom,
	)
	if err != nil {
		return roles.Fandom{}, fmt.Errorf("load genshin roles: %w", err)
	}

	return fandom, nil
}

func (h *Handler) availableRoles(c *botapi.Context, candidates []roles.Role) ([]roles.Role, error) {
	availability, err := h.loadAvailability(c)
	if err != nil {
		return nil, err
	}

	return availability.filter(candidates), nil
}

type roleAvailability struct {
	taken       map[string]struct{}
	reservedIDs map[int64]struct{}
}

func (h *Handler) loadAvailability(c *botapi.Context) (roleAvailability, error) {
	members, err := h.chatMemberService.ListHumanPresentChatMembers(
		c.Background(),
		h.targetChatID,
	)
	if err != nil {
		return roleAvailability{}, fmt.Errorf("list chat members: %w", err)
	}

	reservations, err := h.rolesRepository.ListRoleReservations(c, h.targetChatID)
	if err != nil {
		return roleAvailability{}, fmt.Errorf("list role reservations: %w", err)
	}

	availability := roleAvailability{
		taken:       make(map[string]struct{}, len(members)+len(reservations)),
		reservedIDs: make(map[int64]struct{}, len(reservations)),
	}

	for _, member := range members {
		tag := predicate.NormalizeTag(member.Tag)
		if tag == "" {
			continue
		}

		availability.taken[tag] = struct{}{}
	}

	for _, reservation := range reservations {
		if reservation.Role.ID != 0 {
			availability.reservedIDs[reservation.Role.ID] = struct{}{}
		}

		tag := predicate.NormalizeTag(reservation.Role.Name)
		if tag == "" {
			continue
		}

		availability.taken[tag] = struct{}{}
	}

	return availability, nil
}

func (a roleAvailability) filter(candidates []roles.Role) []roles.Role {
	available := make([]roles.Role, 0, len(candidates))

	for _, role := range candidates {
		if _, ok := a.reservedIDs[role.ID]; ok {
			continue
		}

		if roleTaken(a.taken, role) {
			continue
		}

		available = append(available, role)
	}

	return available
}

func roleTaken(taken map[string]struct{}, role roles.Role) bool {
	if _, ok := taken[predicate.NormalizeTag(role.Name)]; ok {
		return true
	}

	for _, alias := range role.Aliases {
		if _, ok := taken[predicate.NormalizeTag(alias)]; ok {
			return true
		}
	}

	return false
}

func findRoleInFandom(fandom roles.Fandom, categoryID, roleID int64) (roles.Role, bool) {
	for _, category := range fandom.Categories {
		if categoryID != 0 && category.ID != categoryID {
			continue
		}

		for _, role := range category.Roles {
			if role.ID == roleID {
				return role, true
			}
		}
	}

	return roles.Role{}, false
}
