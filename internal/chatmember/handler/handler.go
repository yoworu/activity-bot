package handler

import (
	"activity-bot/internal/action"
	"activity-bot/internal/cctx"
	"activity-bot/internal/chatmember"
	"activity-bot/internal/command"
	"activity-bot/internal/emoji"
	"activity-bot/internal/i18n"
	"activity-bot/internal/info"
	"activity-bot/internal/marriage"
	"activity-bot/internal/option"
	"activity-bot/internal/permission"
	"activity-bot/internal/rule"
	"activity-bot/internal/utils/chatmembers"
	"activity-bot/internal/utils/participant"
	"activity-bot/internal/utils/tghtml"
	"fmt"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/gotd/botapi"
	"github.com/gotd/log"
)

const CategoryChatMember command.Category = "chat_member"

type Handler struct {
	repo            chatmember.Repository
	service         *chatmember.Service
	marriageService *marriage.Service
	updater         *info.Updater
}

func NewHandler(
	repo chatmember.Repository,
	service *chatmember.Service,
	marriageService *marriage.Service,
	updater *info.Updater,
) *Handler {
	return &Handler{
		repo:            repo,
		service:         service,
		marriageService: marriageService,
		updater:         updater,
	}
}

func (h *Handler) Actions() []*command.Action {
	return []*command.Action{
		action.NewCommand(
			"setchatemoji",
			h.SetEmoji,
			i18n.Cmd.ChatMember.SetEmoji.Desc,
			CategoryChatMember,
			option.WithAliases("значок", "чат эмодзи"),
			option.IgnorePermissionCheck(),
			option.WithRules(rule.User().Optional(), rule.Text()),
			option.WithPermission(permission.StatusModerator),
		),
		action.NewCommand(
			"chatemoji",
			h.ShowEmoji,
			i18n.Cmd.ChatMember.ShowEmoji.Desc,
			CategoryChatMember,
			option.WithAliases("значок", "чат эмодзи"),
			option.WithRules(rule.User().Optional()),
		),
		action.NewCommand(
			"delchatemoji",
			h.RemoveEmoji,
			i18n.Cmd.ChatMember.RemoveEmoji.Desc,
			CategoryChatMember,
			option.WithAliases("-значок", "-чат эмодзи"),
			option.WithRules(rule.User().Optional()),
			option.WithPermission(permission.StatusModerator),
			option.IgnorePermissionCheck(),
		),
		action.NewCommand(
			"update",
			h.UpdateChatMembers,
			i18n.Cmd.ChatMember.Update.Desc,
			CategoryChatMember,
			option.WithAliases("обновить чат", "чат обновить"),
		),
		action.NewCommand(
			"updateroles",
			h.UpdateRoles,
			"BETA",
			CategoryChatMember,
		),
		action.NewCommand(
			"setdesc",
			h.SetDescription,
			i18n.Cmd.ChatMember.SetDescription.Desc,
			CategoryChatMember,
			option.WithAliases("описание", "+описание"),
			option.WithRules(rule.Text()),
		),
		action.NewCommand(
			"deldesc",
			h.DeleteDescription,
			i18n.Cmd.ChatMember.DeleteDescription.Desc,
			CategoryChatMember,
			option.WithAliases("-описание"),
		),
		action.NewCommand(
			"setbirthday",
			h.SetChatMemberBirthday,
			i18n.Cmd.ChatMember.Birthday.Set.Desc,
			CategoryChatMember,
			option.WithAliases("др", "+др"),
			option.WithRules(rule.User().Optional(), rule.DateTimeOrDuration()),
			option.WithPermission(permission.StatusModerator),
		),
		action.NewCommand(
			"olds",
			h.OldMembers,
			"Посмотреть дату входа участников во флуд",
			CategoryChatMember,
			option.WithAliases("олды"),
		),
	}
}

func (h *Handler) SetEmoji(c *botapi.Context) error {
	msg := cctx.MustArgsMessage(c)

	emojis := emoji.Extract(msg.OriginalTextHTML())
	if len(emojis) == 0 || len(emojis) > 3 {
		return nil
	}

	emojisString := strings.Join(emojis, "")

	moderator := cctx.MustChatMember(c)
	target, ok := cctx.MustArgs(c).AnyUser()

	if !ok || !moderator.Permitted(cctx.MustPermission(c)) {
		target = moderator
	}

	ch := cctx.MustChat(c)

	if err := h.repo.SetEmoji(c, ch.ID, target.ID(), emojisString); err != nil {
		return fmt.Errorf("set member emoji: %w", err)
	}

	loc := cctx.MustLocalizer(c)

	_, err := c.Reply(loc.T(i18n.Cmd.ChatMember.SetEmoji.Success, i18n.CmdChatMemberSetEmojiSuccessData{
		Emoji: emojisString,
		User:  tghtml.MemberMentionCustom(loc, target, false),
	}), botapi.WithParseMode(botapi.ParseModeHTML))

	return err
}

func (h *Handler) ShowEmoji(c *botapi.Context) error {
	cm := cctx.MustChatMember(c)
	if u, ok := cctx.MustArgs(c).User(); ok {
		cm = u
	}

	emojis := emoji.Extract(cm.Emojis)
	loc := cctx.MustLocalizer(c)

	if len(emojis) == 0 {
		_, err := c.Reply(loc.T(i18n.Cmd.ChatMember.ShowEmoji.NoEmoji, i18n.CmdChatMemberShowEmojiNoEmojiData{
			User: tghtml.MemberMentionCustom(loc, cm, false),
		}), botapi.WithParseMode(botapi.ParseModeHTML))

		return err
	}

	_, err := c.Reply(loc.T(i18n.Cmd.ChatMember.ShowEmoji.Success, i18n.CmdChatMemberShowEmojiSuccessData{
		User:  tghtml.MemberMentionCustom(loc, cm, false),
		Emoji: cm.Emojis,
	}), botapi.WithParseMode(botapi.ParseModeHTML))

	return err
}

func (h *Handler) UpdateChatMembers(c *botapi.Context) error {
	ch := cctx.MustChat(c)

	members, err := participant.GetChatMembers(c.Bot, c, ch.ID)
	if err != nil {
		return fmt.Errorf("get chat members on update: %w", err)
	}

	chatMembers := chatmembers.ExtractMembers(members)

	if err = h.service.SyncChatMembers(c, ch.ID, chatMembers); err != nil {
		return fmt.Errorf("update chat members: %w", err)
	}

	presentUsers := make(map[int64]struct{}, len(chatMembers))
	for _, member := range chatMembers {
		presentUsers[member.User.ID] = struct{}{}
	}

	if err := h.marriageService.DivorceInactiveMarriages(c, ch.ID, presentUsers); err != nil {
		return fmt.Errorf("cleanup marriages: %w", err)
	}

	loc := cctx.MustLocalizer(c)

	if _, err := c.Reply(
		loc.T(i18n.Cmd.ChatMember.Update.Success, nil),
		botapi.WithParseMode(botapi.ParseModeHTML),
	); err != nil {
		return err
	}

	go func() {
		ctx := c.Background()

		updates := []struct {
			name string
			fn   func() error
		}{
			{
				name: "roles",
				fn:   func() error { return h.updater.UpdateRolesPost(c.Background(), ch.ID, c.Bot) },
			},
			{
				name: "birthdays",
				fn:   func() error { return h.updater.UpdateBirthdaysPost(c.Background(), ch.ID, c.Bot) },
			},
			{
				name: "apply",
				fn:   func() error { return h.updater.UpdateApplyPost(c.Background(), ch.ID, c.Bot) },
			},
			{
				name: "rests",
				fn:   func() error { return h.updater.UpdateRestsPost(c.Background(), ch.ID, c.Bot) },
			},
		}

		for i, update := range updates {
			if i > 0 {
				time.Sleep(7 * time.Second)
			}

			if err := update.fn(); err != nil {
				log.For(c.Bot.Logger()).Error(
					ctx,
					"update post",
					log.String("post", update.name),
					log.Error(err),
				)
				return
			}
		}
	}()

	return nil
}

func (h *Handler) UpdateRoles(c *botapi.Context) error {
	chatID, _ := c.Chat()
	return h.updater.UpdateRolesPost(c, int64(chatID.(botapi.ChatIDInt)), c.Bot)
}

func (h *Handler) RemoveEmoji(c *botapi.Context) error {
	moderator := cctx.MustChatMember(c)
	target, ok := cctx.MustArgs(c).User()

	if !ok || !moderator.Permitted(cctx.MustPermission(c)) {
		target = moderator
	}
	ch := cctx.MustChat(c)
	if err := h.repo.SetEmoji(c, ch.ID, target.ID(), ""); err != nil {
		return fmt.Errorf("remove chat emoji: %w", err)
	}

	loc := cctx.MustLocalizer(c)

	_, err := c.Reply(loc.T(i18n.Cmd.ChatMember.RemoveEmoji.Success, i18n.CmdChatMemberRemoveEmojiSuccessData{
		User: tghtml.MemberMentionCustom(loc, target, false),
	}), botapi.WithParseMode(botapi.ParseModeHTML))

	return err
}

func (h *Handler) SetDescription(c *botapi.Context) error {
	msg := cctx.MustArgsMessage(c)
	text := msg.OriginalTextHTML()
	ch := cctx.MustChat(c)
	cm := cctx.MustChatMember(c)
	loc := cctx.MustLocalizer(c)

	if utf8.RuneCountInString(msg.Text) > 500 {
		_, err := c.Reply(loc.T(i18n.Cmd.ChatMember.SetDescription.TooLong, nil))
		return err
	}

	if err := h.repo.SetDescription(c, ch.ID, cm.ID(), text); err != nil {
		return fmt.Errorf("set description: %w", err)
	}

	_, err := c.Reply(loc.T(i18n.Cmd.ChatMember.SetDescription.Success, nil))
	return err
}

func (h *Handler) DeleteDescription(c *botapi.Context) error {
	ch := cctx.MustChat(c)
	cm := cctx.MustChatMember(c)

	if err := h.repo.SetDescription(c, ch.ID, cm.ID(), ""); err != nil {
		return fmt.Errorf("delete description: %w", err)
	}
	loc := cctx.MustLocalizer(c)
	_, err := c.Reply(loc.T(i18n.Cmd.ChatMember.DeleteDescription.Success, nil))

	return err
}

func (h *Handler) SetChatMemberBirthday(c *botapi.Context) error {
	ch := cctx.MustChat(c)
	args := cctx.MustArgs(c)
	u, ok := args.User()
	if !ok {
		u = cctx.MustChatMember(c)
	}

	bd, ok := args.DateTime()
	if !ok {
		return nil
	}

	if err := h.repo.SetBirthday(c, ch.ID, u.ID(), bd); err != nil {
		return fmt.Errorf("set birthday: %w", err)
	}

	if err := h.updater.UpdateBirthdaysPost(c, ch.ID, c.Bot); err != nil {
		return fmt.Errorf("update birthdays post: %w", err)
	}
	loc := cctx.MustLocalizer(c)

	_, err := c.Reply(loc.T(i18n.Cmd.ChatMember.Birthday.Set.Success, i18n.CmdChatMemberBirthdaySetSuccessData{
		User: tghtml.MemberLink(loc, ch, u),
	}), botapi.WithParseMode(botapi.ParseModeHTML), botapi.DisableWebPagePreview())

	return err

}

func (h *Handler) OldMembers(c *botapi.Context) error {
	ch := cctx.MustChat(c)
	loc := cctx.MustLocalizer(c)

	cms, err := h.repo.List(c, chatmember.Filter{
		ChatID: ch.ID,
		IsBot: chatmember.OptionalBool{
			Bool:  false,
			Valid: true,
		},
		Left: chatmember.OptionalBool{
			Bool:  false,
			Valid: true,
		},
	})
	if err != nil {
		return fmt.Errorf("old members: %w", err)
	}

	sb := strings.Builder{}
	sb.WriteString("Даты вступления участников\n\n")
	sb.WriteString("<blockquote expandable>")
	for i, cm := range cms {
		sb.WriteString(fmt.Sprintf("%d. ", i+1) + tghtml.MemberLink(loc, ch, cm) + " " + tghtml.RelativeDateTime(cm.JoinedAt, time.Now()))
		if i < len(cms)-1 {
			sb.WriteString("\n")
		}
	}
	sb.WriteString("</blockquote>")

	_, err = c.Reply(sb.String(),
		botapi.WithParseMode(botapi.ParseModeHTML),
		botapi.DisableWebPagePreview())

	return err
}
