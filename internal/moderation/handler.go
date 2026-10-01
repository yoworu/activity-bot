package moderation

import (
	"activity-bot/internal/action"
	"activity-bot/internal/chatmember"
	"activity-bot/internal/command"
	"activity-bot/internal/i18n"
	"activity-bot/internal/info"
	"activity-bot/internal/option"
	"activity-bot/internal/permission"
	"activity-bot/internal/rule"
	"fmt"

	"github.com/gotd/botapi"
)

const CategoryModeration command.Category = "moderation"

type Handler struct {
	service           *Service
	chatMemberService *chatmember.Service
	roleUpdater       *info.Updater
}

func NewHandler(
	service *Service,
	cms *chatmember.Service,
	roleUpdater *info.Updater,
) *Handler {
	return &Handler{
		service:           service,
		chatMemberService: cms,
		roleUpdater:       roleUpdater,
	}
}

func (h *Handler) Actions() []*command.Action {
	return []*command.Action{
		action.NewCommand(
			"setrole",
			h.SetRole,
			i18n.Cmd.Moderation.SetRole.Desc,
			CategoryModeration,
			option.WithPermission(permission.StatusModerator),
			option.WithAliases("+роль", "роль"),
			option.WithRules(
				rule.User(),
				rule.Text().Validate(isValidRoleString),
			),
		),

		action.NewCommand(
			"setroleadmin",
			h.SetRoleAdmin,
			i18n.Cmd.Moderation.SetRoleAdmin.Desc,
			CategoryModeration,
			option.WithPermission(permission.StatusCoOwner),
			option.WithAliases("ароль", "адмроль"),
			option.WithRules(
				rule.User(),
				rule.Text().Validate(isValidRoleString),
			),
		),
		action.NewCallbackPrefix(
			"toggleadmin",
			"toggle_admin_right:",
			h.OnRightToggleCallback,
			CategoryModeration,
			option.WithPermission(permission.StatusCoOwner),
		),
		action.NewCommand(
			"roles",
			h.ListRoles,
			i18n.Cmd.Moderation.ListRoles.Desc,
			CategoryModeration,
			option.WithAliases("роли"),
		),
		action.NewCommand(
			"ban",
			h.Ban,
			i18n.Cmd.Moderation.Ban.Desc,
			CategoryModeration,
			option.WithPermission(permission.StatusSeniorAdmin),
			option.WithAliases("бан", "кик"),
			option.WithRules(
				rule.User(),
				rule.DateTimeOrDuration().Optional(),
				rule.Text().Optional(),
			),
		),

		action.NewCommand(
			"mute",
			h.Mute,
			i18n.Cmd.Moderation.Mute.Desc,
			CategoryModeration,
			option.WithPermission(permission.StatusAdmin),
			option.WithAliases("мут"),
			option.WithRules(
				rule.User(),
				rule.DateTimeOrDuration().Optional(),
				rule.Text().Optional(),
			),
		),

		action.NewCommand(
			"unban",
			h.Unban,
			i18n.Cmd.Moderation.Unban.Desc,
			CategoryModeration,
			option.WithPermission(permission.StatusSeniorAdmin),
			option.WithAliases("-мут", "разбан", "размут", "снять мут", "говори"),
			option.WithRules(
				rule.User(),
			),
		),

		action.NewCommand(
			"kick",
			h.Kick,
			i18n.Cmd.Moderation.Kick.Desc,
			CategoryModeration,
			option.WithPermission(permission.StatusSeniorAdmin),
			option.WithAliases("кик"),
			option.WithRules(
				rule.User(),
				rule.DateTimeOrDuration().Optional(),
				rule.Text().Optional(),
			),
		),

		action.NewCommand(
			"promote",
			h.Promote,
			i18n.Cmd.Moderation.Promote.Desc,
			CategoryModeration,
			option.WithPermission(permission.StatusCoOwner),
			option.WithAliases("повысить"),
			option.WithRules(rule.User(), rule.Number().Optional()),
		),
		action.NewCommand(
			"demote",
			h.Demote,
			i18n.Cmd.Moderation.Demote.Desc,
			CategoryModeration,
			option.WithPermission(permission.StatusCoOwner),
			option.WithAliases("понизить"),
			option.WithRules(rule.User(), rule.Number().Optional()),
		),
		action.NewCommand(
			"setstatus",
			h.SetStatus,
			i18n.Cmd.Moderation.SetStatus.Desc,
			CategoryModeration,
			option.WithPermission(permission.StatusCoOwner),
			option.AllowDev(),
			option.WithAliases("статус", "админ", "+админ", "права"),
			option.WithRules(rule.User().Optional(), rule.Number()),
		),

		action.NewCommand(
			"admins",
			h.ListAdmins,
			i18n.Cmd.Moderation.ListAdmins.Desc,
			CategoryModeration,
			option.WithAliases("админы", "кто здесь власть"),
		),
		action.NewCommand(
			"warn",
			h.Warn,
			i18n.Cmd.Moderation.Warn.Desc,
			CategoryModeration,
			option.WithPermission(permission.StatusAdmin),
			option.WithAliases("варн", "пред"),
			option.WithRules(
				rule.User(),
				rule.DateTimeOrDuration().Optional(),
				rule.Text().Optional(),
			),
		),
		action.NewCommand(
			"unwarn",
			h.Unwarn,
			i18n.Cmd.Moderation.Unwarn.Desc,
			CategoryModeration,
			option.WithPermission(permission.StatusSeniorAdmin),
			option.WithAliases("-пред", "снять пред", "-варн", "снять варн"),
			option.WithRules(
				rule.User(),
			),
		),
		action.NewCommand(
			"clearwarns",
			h.ClearWarns,
			i18n.Cmd.Moderation.ClearWarns.Desc,
			CategoryModeration,
			option.WithPermission(permission.StatusAdmin),
			option.WithAliases("очистить преды", "очистить варны"),
			option.WithRules(
				rule.User(),
			),
		),
		action.NewCommand(
			"warns",
			h.ShowWarns,
			i18n.Cmd.Moderation.ShowWarns.Desc,
			CategoryModeration,
			option.WithPermission(permission.StatusModerator),
			option.WithAliases("варны"),
			option.WithRules(
				rule.User().Optional(),
			),
		),
		action.NewCommand(
			"maxwarns",
			h.ShowMaxWarns,
			i18n.Cmd.Moderation.MaxWarns.Desc,
			CategoryModeration,
			option.WithAliases("макс преды", "макс варны"),
		),

		action.NewCommand(
			"setmaxwarns",
			h.SetMaxWarns,
			i18n.Cmd.Moderation.SetMaxWarns.Desc,
			CategoryModeration,
			option.WithPermission(permission.StatusCoOwner),
			option.WithAliases("max_warns", "макс преды", "макс варны"),
			option.WithRules(
				rule.Number(),
			),
		),

		action.NewCommand(
			"warnlist",
			h.WarnList,
			i18n.Cmd.Moderation.WarnList.Desc,
			CategoryModeration,
			option.WithPermission(permission.StatusModerator),
			option.WithAliases("варнлист"),
		),

		action.NewCommand(
			"delmessage",
			h.DeleteMessage,
			i18n.Cmd.Moderation.DeleteMessage.Desc,
			CategoryModeration,
			option.WithPermission(permission.StatusAdmin),
			option.WithAliases("-смс"),
		),

		action.NewCommand(
			"chatoff",
			h.OffChat,
			i18n.Cmd.Moderation.Chatoff.Desc,
			CategoryModeration,
			option.WithPermission(permission.StatusSeniorAdmin),
			option.WithAliases("-чат"),
		),

		action.NewCommand(
			"chaton",
			h.OnChat,
			i18n.Cmd.Moderation.Chaton.Desc,
			CategoryModeration,
			option.WithPermission(permission.StatusSeniorAdmin),
			option.WithAliases("+чат"),
		),
	}
}

func (h *Handler) DeleteMessage(c *botapi.Context) error {
	msg := c.Message()
	if msg == nil {
		return nil
	}
	if msg.ReplyToMessage == nil {
		return nil
	}
	chatID, _ := c.Chat()

	if err := c.Bot.DeleteMessage(c, chatID, msg.ReplyToMessage.MessageID); err != nil {
		return fmt.Errorf("delete reply msg: %w", err)
	}

	if err := c.Bot.DeleteMessage(c, chatID, msg.MessageID); err != nil {
		return fmt.Errorf("delete current msg: %w", err)
	}

	return nil
}

func (h *Handler) OffChat(c *botapi.Context) error {
	chatID, _ := c.Chat()

	err := c.Bot.SetChatPermissions(c, chatID, botapi.ChatPermissions{
		CanSendMessages:       false,
		CanSendAudios:         false,
		CanSendDocuments:      false,
		CanSendPhotos:         false,
		CanSendVideos:         false,
		CanSendVideoNotes:     false,
		CanSendVoiceNotes:     false,
		CanSendPolls:          false,
		CanSendOtherMessages:  false,
		CanAddWebPagePreviews: false,
		CanChangeInfo:         false,
		CanInviteUsers:        false,
		CanPinMessages:        false,
		CanManageTopics:       false,
	})
	if err != nil {
		return fmt.Errorf("off chat: %w", err)
	}

	return nil
}

func (h *Handler) OnChat(c *botapi.Context) error {
	chatID, _ := c.Chat()

	err := c.Bot.SetChatPermissions(c, chatID, botapi.ChatPermissions{
		CanSendMessages:       true,
		CanSendAudios:         true,
		CanSendDocuments:      true,
		CanSendPhotos:         true,
		CanSendVideos:         true,
		CanSendVideoNotes:     true,
		CanSendVoiceNotes:     true,
		CanSendPolls:          true,
		CanSendOtherMessages:  true,
		CanAddWebPagePreviews: true,
		CanChangeInfo:         false,
		CanInviteUsers:        false,
		CanPinMessages:        false,
		CanManageTopics:       false,
	})
	if err != nil {
		return fmt.Errorf("on chat: %w", err)
	}

	return nil
}
