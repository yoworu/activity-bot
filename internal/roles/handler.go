package roles

import (
	"activity-bot/internal/action"
	"activity-bot/internal/cctx"
	"activity-bot/internal/command"
	"activity-bot/internal/i18n"
	"activity-bot/internal/info/genshin"
	"activity-bot/internal/option"
	"activity-bot/internal/permission"
	"activity-bot/internal/predicate"
	"activity-bot/internal/rule"
	"fmt"

	"github.com/gotd/botapi"
)

type Handler struct {
	repo Repository
}

func NewHandler(repo Repository) *Handler {
	return &Handler{
		repo: repo,
	}
}

const RolesCategory command.Category = "roles"

func (h *Handler) Actions() []*command.Action {
	return []*command.Action{
		action.NewCommand(
			"rolesgenshin",
			h.ImportGenshin,
			i18n.Cmd.Roles.Genshin.Desc,
			RolesCategory,
			option.WithAliases("роли геншин"),
			option.WithPermission(permission.StatusCoOwner),
		),
		action.NewCommand(
			"reserve",
			h.ReserveRole,
			i18n.Cmd.Roles.Reserve.Desc,
			RolesCategory,
			option.WithAliases("бронь"),
			option.WithRules(rule.Number().Optional(), rule.Text()),
			option.WithPermission(permission.StatusAdmin),
		),
		action.NewCommand(
			"free",
			h.FreeRole,
			i18n.Cmd.Roles.Free.Desc,
			RolesCategory,
			option.WithAliases("-бронь", "снять бронь"),
			option.WithRules(rule.Text()),
			option.WithPermission(permission.StatusAdmin),
		),
		action.NewCommand(
			"addfandom",
			h.AddFandom,
			i18n.Cmd.Roles.AddFandom.Desc,
			RolesCategory,
			option.WithAliases("показать роли"),
		),
	}
}

func (h *Handler) ReserveRole(c *botapi.Context) error {
	ch := cctx.MustChat(c)
	args := cctx.MustArgs(c)
	text, ok := args.Text()
	if !ok {
		return nil
	}
	number, ok := args.Number()
	if !ok {
		return nil
	}
	role, err := h.repo.GetRoleByNameOrAlias(c, ch.ID, "Genshin Impact", predicate.NormalizeTag(text))

	if err != nil {
		return fmt.Errorf("reserve role: %w", err)
	}

	if err := h.repo.CreateRoleReservation(c, ch.ID, number, role.ID); err != nil {
		return fmt.Errorf("create role reservation: %w", err)
	}

	loc := cctx.MustLocalizer(c)

	_, err = c.Reply(loc.T(i18n.Cmd.Roles.Reserve.Success, i18n.CmdRolesReserveSuccessData{
		Role: role.Name,
	}))

	return err
}

func (h *Handler) FreeRole(c *botapi.Context) error {
	ch := cctx.MustChat(c)
	args := cctx.MustArgs(c)
	text, ok := args.Text()
	if !ok {
		return nil
	}
	role, err := h.repo.GetRoleByNameOrAlias(c, ch.ID, "Genshin Impact", predicate.NormalizeTag(text))

	if err != nil {
		return fmt.Errorf("reserve role: %w", err)
	}

	if err := h.repo.DeleteRoleReservation(c, ch.ID, role.ID); err != nil {
		return fmt.Errorf("create role reservation: %w", err)
	}

	loc := cctx.MustLocalizer(c)

	_, err = c.Reply(loc.T(i18n.Cmd.Roles.Free.Success, i18n.CmdRolesFreeSuccessData{
		Role: role.Name,
	}))

	return err
}

func (h *Handler) ImportGenshin(c *botapi.Context) error {
	ch := cctx.MustChat(c)

	if err := h.repo.CreateRoleTemplate(c, ch.ID, "Genshin Impact", GenshinRoles()); err != nil {
		return fmt.Errorf("create role template: %w", err)
	}

	loc := cctx.MustLocalizer(c)

	_, err := c.Reply(loc.T(i18n.Cmd.Roles.Genshin.Success, nil))
	return err
}

func (h *Handler) AddFandom(c *botapi.Context) error {
	ch := cctx.MustChat(c)
	chatID, _ := c.Chat()

	fandoms, err := h.repo.ListRoleTemplates(c, ch.ID)
	if err != nil {
		return fmt.Errorf("list role templates: %w", err)
	}

	if len(fandoms) == 0 {
		loc := cctx.MustLocalizer(c)

		_, err = c.Reply(loc.T(i18n.Cmd.Roles.AddFandom.Empty, nil))
		return err
	}

	opts := []botapi.SendOption{
		botapi.WithParseMode(botapi.ParseModeHTML),
		botapi.DisableWebPagePreview(),
	}

	for i, text := range RenderFandomTemplate(fandoms) {
		if i == 0 {
			_, err = c.Reply(text, opts...)
		} else {
			_, err = c.Bot.SendMessage(c, chatID, text, opts...)
		}

		if err != nil {
			return fmt.Errorf("send fandom template: %w", err)
		}
	}

	return nil
}

func GenshinRoles() []Category {
	result := make([]Category, 0, len(genshin.Categories))

	for _, category := range genshin.Categories {
		result = append(result, Category{
			Name:  category.Name,
			Roles: mapRoles(category.Roles),
		})
	}

	return result
}

func mapRoles(source []genshin.Role) []Role {
	result := make([]Role, 0, len(source))

	for _, role := range source {
		result = append(result, Role{
			Name:    role.Name,
			Emoji:   role.Emoji,
			Aliases: role.Aliases,
		})
	}

	return result
}
