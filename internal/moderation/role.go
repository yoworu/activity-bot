package moderation

import (
	"activity-bot/internal/cctx"
	"activity-bot/internal/chatmember"
	"activity-bot/internal/i18n"
	"activity-bot/internal/utils/tghtml"
	"errors"
	"fmt"
	"slices"
	"strings"
	"unicode/utf8"

	"github.com/gotd/td/tgerr"

	"github.com/gotd/botapi"
)

func isValidRoleString(text string) bool {
	return utf8.RuneCountInString(text) <= 16
}

func (h *Handler) SetRole(c *botapi.Context) error {
	cm := cctx.MustChatMember(c)
	args := cctx.MustArgs(c)
	ch := cctx.MustChat(c)

	u, ok := args.User()
	if ok {
		cm = u
	}

	newRole, ok := args.Text()
	if !ok {
		return nil
	}

	roleName, _, _ := strings.Cut(newRole, "|")
	roleName = strings.TrimSpace(roleName)

	if err := h.chatMemberService.UpdateTag(c, ch.ID, cm.ID(), newRole); err != nil {
		return fmt.Errorf("set role upd tag: %w", err)
	}

	loc := cctx.MustLocalizer(c)

	if err := c.Bot.SetChatMemberTag(c, botapi.ID(ch.ID), cm.ID(), newRole); err != nil {
		if tgErr, ok := errors.AsType[*tgerr.Error](err); ok && tgErr.Type == "CHAT_CREATOR_REQUIRED" {
			if cm.IsOwner() {
				_, err = c.Reply(loc.T(i18n.Cmd.Moderation.SetRole.ChatCreator, nil))
			} else {
				_, err = c.Reply(loc.T(i18n.Cmd.Moderation.SetRole.ChatAdmin, nil))
			}

			return err
		}

		if tgErr, ok := errors.AsType[*tgerr.Error](err); ok && tgErr.Type == "CHAT_ADMIN_REQUIRED" {
			_, err = c.Reply(loc.T(i18n.Cmd.Moderation.SetRole.NoRights, nil))
			return err
		}

		return fmt.Errorf("set role set tag: %w", err)
	}

	fandom, err := h.rolesRepository.GetRoleTemplate(
		c,
		ch.ID,
		"Genshin Impact",
	)
	if err != nil {
		return fmt.Errorf("load genshin roles: %w", err)
	}

	for _, category := range fandom.Categories {
		for _, role := range category.Roles {
			if !strings.EqualFold(strings.TrimSpace(role.Name), roleName) {
				continue
			}

			if role.Emoji != "" {
				if err := h.chatMemberService.SetEmoji(c, ch.ID, cm.ID(), role.Emoji); err != nil {
					return fmt.Errorf("set role emoji: %w", err)
				}
			}

			break
		}
	}

	if _, err := c.Reply(
		loc.T(i18n.Cmd.Moderation.SetRole.Set, i18n.CmdModerationSetRoleSetData{
			User:    tghtml.MemberMention(loc, ch, cm),
			Changed: newRole,
		}),
		botapi.WithParseMode(botapi.ParseModeHTML),
	); err != nil {
		return fmt.Errorf("set role reply: %w", err)
	}

	chatID, _ := c.Chat()

	if err := h.roleUpdater.UpdateRolesPost(c, int64(chatID.(botapi.ChatIDInt)), c.Bot); err != nil {
		return fmt.Errorf("set role: %w", err)
	}

	return nil
}

type adminRightItem struct {
	Key    string
	LocKey i18n.MessageID
}

var adminRightsList = []adminRightItem{
	{Key: "can_manage_chat", LocKey: i18n.Cmd.Moderation.Rights.CanManageChat},
	{Key: "can_delete_messages", LocKey: i18n.Cmd.Moderation.Rights.CanDeleteMessages},
	{Key: "can_restrict_members", LocKey: i18n.Cmd.Moderation.Rights.CanRestrictMembers},
	{Key: "can_promote_members", LocKey: i18n.Cmd.Moderation.Rights.CanPromoteMembers},
	{Key: "can_change_info", LocKey: i18n.Cmd.Moderation.Rights.CanChangeInfo},
	{Key: "can_invite_users", LocKey: i18n.Cmd.Moderation.Rights.CanInviteUsers},
	{Key: "can_pin_messages", LocKey: i18n.Cmd.Moderation.Rights.CanPinMessages},
}

func buildRightsKeyboard(loc *i18n.Localizer, targetUserID int64, rights botapi.ChatAdminRights) *botapi.InlineKeyboardMarkup {
	const columns = 2
	var rows [][]botapi.InlineKeyboardButton
	var currentRow []botapi.InlineKeyboardButton

	for _, r := range adminRightsList {
		enabled := isRightEnabled(rights, r.Key)
		icon := "❌"
		if enabled {
			icon = "✅"
		}

		title := loc.T(r.LocKey, nil)
		btnText := fmt.Sprintf("%s %s", icon, title)
		callbackData := fmt.Sprintf("toggle_admin_right:%d:%s", targetUserID, r.Key)

		btn := botapi.InlineButtonData(btnText, callbackData)
		currentRow = append(currentRow, btn)

		if len(currentRow) == columns {
			rows = append(rows, currentRow)
			currentRow = nil
		}
	}

	if len(currentRow) > 0 {
		rows = append(rows, currentRow)
	}

	return botapi.InlineKeyboard(rows...)
}

func isRightEnabled(rights botapi.ChatAdminRights, key string) bool {
	switch key {
	case "can_manage_chat":
		return rights.CanManageChat
	case "can_delete_messages":
		return rights.CanDeleteMessages
	case "can_restrict_members":
		return rights.CanRestrictMembers
	case "can_promote_members":
		return rights.CanPromoteMembers
	case "can_change_info":
		return rights.CanChangeInfo
	case "can_invite_users":
		return rights.CanInviteUsers
	case "can_pin_messages":
		return rights.CanPinMessages
	case "can_manage_topics":
		return rights.CanManageTopics
	default:
		return false
	}
}

func extractAdminRights(member botapi.ChatMember) botapi.ChatAdminRights {
	if admin, ok := member.(*botapi.ChatMemberAdministrator); ok {
		return botapi.ChatAdminRights{
			IsAnonymous:         admin.IsAnonymous,
			CanManageChat:       admin.CanManageChat,
			CanDeleteMessages:   admin.CanDeleteMessages,
			CanManageVideoChats: admin.CanManageVideoChats,
			CanRestrictMembers:  admin.CanRestrictMembers,
			CanPromoteMembers:   admin.CanPromoteMembers,
			CanChangeInfo:       admin.CanChangeInfo,
			CanInviteUsers:      admin.CanInviteUsers,
			CanPostMessages:     admin.CanPostMessages,
			CanEditMessages:     admin.CanEditMessages,
			CanPinMessages:      admin.CanPinMessages,
			CustomTitle:         admin.CustomTitle,
		}
	}

	return botapi.ChatAdminRights{
		CanManageChat: true,
	}
}

func (h *Handler) SetRoleAdmin(c *botapi.Context) error {
	cm := cctx.MustChatMember(c)
	args := cctx.MustArgs(c)
	ch := cctx.MustChat(c)

	u, ok := args.User()
	if ok {
		cm = u
	}

	newRole, ok := args.Text()
	if !ok {
		return nil
	}

	if err := h.chatMemberService.UpdateTag(c, ch.ID, cm.ID(), newRole); err != nil {
		return fmt.Errorf("set role admin upd tag: %w", err)
	}

	loc := cctx.MustLocalizer(c)
	if err := c.Bot.SetChatMemberTag(c, botapi.ID(ch.ID), cm.ID(), newRole); err != nil {
		if tgErr, ok := errors.AsType[*tgerr.Error](err); ok && tgErr.Type == "CHAT_CREATOR_REQUIRED" {
			if cm.IsOwner() {
				_, err = c.Reply(loc.T(i18n.Cmd.Moderation.SetRoleAdmin.ChatCreator, nil))
			} else {
				_, err = c.Reply(loc.T(i18n.Cmd.Moderation.SetRoleAdmin.ChatAdmin, nil))
			}

			return err
		}

		if tgErr, ok := errors.AsType[*tgerr.Error](err); ok && tgErr.Type == "CHAT_ADMIN_REQUIRED" {
			_, err = c.Reply(loc.T(i18n.Cmd.Moderation.SetRoleAdmin.NoRights, nil))

			return err
		}

		return fmt.Errorf("set role admin set tag: %w", err)
	}

	member, err := c.Bot.GetChatMember(c, botapi.ID(ch.ID), cm.ID())
	if err != nil {
		return fmt.Errorf("set role admin get chat member: %w", err)
	}

	currentRights := extractAdminRights(member)
	currentRights.CustomTitle = newRole

	if err := c.Bot.PromoteChatMember(c, botapi.ID(ch.ID), cm.ID(), currentRights); err != nil {
		return fmt.Errorf("set role admin promote chat member: %w", err)
	}

	kb := buildRightsKeyboard(loc, cm.ID(), currentRights)
	if _, err := c.Reply(
		loc.T(i18n.Cmd.Moderation.SetRole.Set, i18n.CmdModerationSetRoleSetData{
			User:    tghtml.MemberMention(loc, ch, cm),
			Changed: newRole,
		}),
		botapi.WithParseMode(botapi.ParseModeHTML),
		botapi.WithReplyMarkup(kb),
	); err != nil {
		return fmt.Errorf("set role admin reply: %w", err)
	}

	return nil
}

func (h *Handler) OnRightToggleCallback(c *botapi.Context) error {
	cb := c.Update.CallbackQuery
	if cb == nil {
		return nil
	}

	parts := strings.Split(cb.Data, ":")
	if len(parts) < 3 || parts[0] != "toggle_admin_right" {
		return nil
	}

	var targetUserID int64
	_, _ = fmt.Sscanf(parts[1], "%d", &targetUserID)
	rightKey := parts[2]

	ch := cctx.MustChat(c)
	loc := cctx.MustLocalizer(c)

	member, err := c.Bot.GetChatMember(c, botapi.ID(ch.ID), targetUserID)
	if err != nil {
		return fmt.Errorf("get chat member failed: %w", err)
	}

	memberAdmin, ok := member.(*botapi.ChatMemberAdministrator)
	if !ok {
		return errors.New("not a chat admin")
	}

	toggleRightAdminMember(memberAdmin, rightKey)
	adminRights := extractAdminRights(memberAdmin)

	if err := c.Bot.PromoteChatMember(c, botapi.ID(ch.ID), targetUserID, adminRights); err != nil {
		_ = c.AnswerCallback(botapi.WithCallbackText(loc.T(i18n.Cmd.Moderation.Rights.UpdateFailed, nil)))
		return fmt.Errorf("toggle right promote error: %w", err)
	}

	newKb := buildRightsKeyboard(loc, targetUserID, adminRights)
	chatID, _ := c.Chat()

	if _, err := c.Bot.EditMessageReplyMarkup(c, chatID, cb.Message.MessageID, newKb); err != nil {
		return fmt.Errorf("edit message markup: %w", err)
	}

	return c.AnswerCallback(botapi.WithCallbackText(loc.T(i18n.Cmd.Moderation.Rights.Updated, nil)))
}

func toggleRightAdminMember(m *botapi.ChatMemberAdministrator, key string) {
	switch key {
	case "can_manage_chat":
		m.CanManageChat = !m.CanManageChat
	case "can_delete_messages":
		m.CanDeleteMessages = !m.CanDeleteMessages
	case "can_restrict_members":
		m.CanRestrictMembers = !m.CanRestrictMembers
	case "can_promote_members":
		m.CanPromoteMembers = !m.CanPromoteMembers
	case "can_change_info":
		m.CanChangeInfo = !m.CanChangeInfo
	case "can_invite_users":
		m.CanInviteUsers = !m.CanInviteUsers
	case "can_pin_messages":
		m.CanPinMessages = !m.CanPinMessages
	}
}

func (h *Handler) ListRoles(c *botapi.Context) error {
	ch := cctx.MustChat(c)
	loc := cctx.MustLocalizer(c)

	members, err := h.chatMemberService.ListHumanPresentChatMembers(c, ch.ID)
	if err != nil {
		return fmt.Errorf("list roles members: %w", err)
	}

	var withRoles []chatmember.ChatMember
	for _, member := range members {
		if member.Tag != "" {
			withRoles = append(withRoles, member)
		}
	}

	if len(withRoles) == 0 {
		_, err := c.Reply(loc.T(i18n.Cmd.Moderation.ListRoles.Empty, nil))
		return err
	}

	slices.SortFunc(withRoles, func(a, b chatmember.ChatMember) int {
		return strings.Compare(a.Tag, b.Tag)
	})

	var text strings.Builder

	text.WriteString(loc.T(i18n.Cmd.Moderation.ListRoles.Header, nil))
	text.WriteString("\n\n")
	text.WriteString("<blockquote expandable>")

	for i, member := range withRoles {
		if i > 0 {
			text.WriteByte('\n')
		}

		text.WriteString(fmt.Sprintf("%d. ", i+1))
		text.WriteString(tghtml.MemberLink(loc, ch, member))
		if member.User.Username != "" {
			text.WriteString(fmt.Sprintf(" <code>@%s</code>", member.User.Username))
		}
	}
	text.WriteString("</blockquote>")

	_, err = c.Reply(
		text.String(),
		botapi.WithParseMode(botapi.ParseModeHTML),
		botapi.DisableWebPagePreview(),
	)

	return err
}
