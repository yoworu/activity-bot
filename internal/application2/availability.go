package application2

import (
	"activity-bot/internal/predicate"
	"activity-bot/internal/roles"
	"fmt"

	"github.com/gotd/botapi"
)

const genshinFandom = "Genshin Impact"

func (h *Handler) loadFandom(c *botapi.Context) (roles.Fandom, error) {
	fandom, err := h.rolesRepository.GetRoleTemplate(
		c,
		h.targetChatID,
		genshinFandom,
	)
	if err == nil {
		return fandom, nil
	}

	fandoms, listErr := h.rolesRepository.ListRoleTemplates(c, h.targetChatID)
	if listErr != nil {
		return roles.Fandom{}, fmt.Errorf("load roles: %w", err)
	}
	if len(fandoms) == 0 {
		return roles.Fandom{}, fmt.Errorf("load roles: %w", err)
	}

	return fandoms[0], nil
}

func (h *Handler) availableCategories(c *botapi.Context) ([]roles.Category, error) {
	fandom, err := h.loadFandom(c)
	if err != nil {
		return nil, err
	}

	availability, err := h.loadAvailability(c)
	if err != nil {
		return nil, err
	}

	categories := make([]roles.Category, 0, len(fandom.Categories))

	for _, category := range fandom.Categories {
		if len(availability.filter(category.Roles)) == 0 {
			continue
		}

		categories = append(categories, category)
	}

	return categories, nil
}

func (h *Handler) availableRoles(
	c *botapi.Context,
	candidates []roles.Role,
) ([]roles.Role, error) {
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
