package handler

import (
	"activity-bot/internal/action"
	"activity-bot/internal/cctx"
	"activity-bot/internal/command"
	"activity-bot/internal/emoji"
	"activity-bot/internal/i18n"
	"activity-bot/internal/option"
	"activity-bot/internal/rule"
	"activity-bot/internal/user"
	"activity-bot/internal/utils/tghtml"
	"fmt"
	"strings"

	"github.com/gotd/botapi"
)

const CategoryUser command.Category = "user"

type Handler struct {
	repo user.Repository
}

func NewHandler(repo user.Repository) *Handler {
	return &Handler{repo: repo}
}

func (h *Handler) Actions() []*command.Action {
	return []*command.Action{
		action.NewCommand(
			"setemoji",
			h.SetEmoji,
			i18n.Cmd.User.SetEmoji.Desc,
			CategoryUser,
			option.WithAliases("мой эмодзи", "мой эмоджи", "эмоджи", "эмодзи"),
			option.WithScope(command.ScopeAny),
			option.WithRules(rule.Text()),
		),
		action.NewCommand(
			"delemoji",
			h.RemoveEmoji,
			i18n.Cmd.User.RemoveEmoji.Desc,
			CategoryUser,
			option.WithAliases("-эмодзи", "-эмоджи"),
		),
		action.NewCommand(
			"emoji",
			h.ShowEmoji,
			i18n.Cmd.User.ShowEmoji.Desc,
			CategoryUser,
			option.WithAliases("мой эмодзи", "мой эмоджи", "эмоджи", "эмодзи"),
			option.WithScope(command.ScopeAny),
		),
		action.NewCommand(
			"gender",
			h.ShowGender,
			i18n.Cmd.User.ShowGender.Desc,
			CategoryUser,
			option.WithAliases("мой пол", "пол"),
			option.WithRules(rule.User().Optional()),
		),
		action.NewCommand(
			"setgender",
			h.SetGender,
			i18n.Cmd.User.SetGender.Desc,
			CategoryUser,
			option.WithRules(rule.Text()),
			option.WithAliases("мой пол", "пол"),
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

	cm := cctx.MustChatMember(c)

	if err := h.repo.SetEmoji(c, cm.ID(), emojisString); err != nil {
		return fmt.Errorf("set emoji: %w", err)
	}

	loc := cctx.MustLocalizer(c)

	_, err := c.Reply(loc.T(i18n.Cmd.User.SetEmoji.Success, i18n.CmdUserSetEmojiSuccessData{
		Emoji: emojisString,
		User:  tghtml.MemberMentionCustom(loc, cm, false),
	}), botapi.WithParseMode(botapi.ParseModeHTML))

	return err
}

func (h *Handler) RemoveEmoji(c *botapi.Context) error {
	cm := cctx.MustChatMember(c)

	if err := h.repo.SetEmoji(c, cm.ID(), ""); err != nil {
		return fmt.Errorf("remove emoji: %w", err)
	}

	loc := cctx.MustLocalizer(c)

	_, err := c.Reply(loc.T(i18n.Cmd.User.RemoveEmoji.Success, i18n.CmdUserRemoveEmojiSuccessData{
		User: tghtml.MemberMentionCustom(loc, cm, false),
	}), botapi.WithParseMode(botapi.ParseModeHTML))

	return err
}

func (h *Handler) ShowEmoji(c *botapi.Context) error {
	cm := cctx.MustChatMember(c)
	emojis := emoji.Extract(cm.User.Emojis)
	loc := cctx.MustLocalizer(c)

	if len(emojis) == 0 {
		_, err := c.Reply(loc.T(i18n.Cmd.User.ShowEmoji.NoEmoji, i18n.CmdUserShowEmojiNoEmojiData{
			User: tghtml.MemberMentionCustom(loc, cm, false),
		}), botapi.WithParseMode(botapi.ParseModeHTML))

		return err
	}

	_, err := c.Reply(loc.T(i18n.Cmd.User.ShowEmoji.Success, i18n.CmdUserShowEmojiSuccessData{
		User:  tghtml.MemberMentionCustom(loc, cm, false),
		Emoji: cm.User.Emojis,
	}), botapi.WithParseMode(botapi.ParseModeHTML))

	return err
}

func (h *Handler) SetGender(c *botapi.Context) error {
	cm := cctx.MustChatMember(c)
	loc := cctx.MustLocalizer(c)

	text, ok := cctx.MustArgs(c).Text()
	if !ok {
		return nil
	}

	text = strings.ToLower(text)
	var gender user.Gender
	switch text {
	case "m", "male", "м", "мужской":
		gender = user.GenderMale
	case "f", "female", "ж", "женский":
		gender = user.GenderFemale
	case "u", "unknown", "н", "неизвестен":
		gender = user.GenderUnknown
	default:
		return nil
	}

	if err := h.repo.SetGender(c, cm.ID(), gender); err != nil {
		return fmt.Errorf("set gender: %w", err)
	}

	_, err := c.Reply(loc.T(i18n.Cmd.User.SetGender.Success, i18n.CmdUserSetGenderSuccessData{
		Gender: loc.T(i18n.System.Gender, nil, i18n.WithGender(gender)),
	}))

	return err
}

func (h *Handler) ShowGender(c *botapi.Context) error {
	cm := cctx.MustChatMember(c)
	if u, ok := cctx.MustArgs(c).User(); ok {
		cm = u
	}

	loc := cctx.MustLocalizer(c)
	ch := cctx.MustChat(c)
	_, err := c.Reply(loc.T(i18n.Cmd.User.ShowGender.Success, i18n.CmdUserShowGenderSuccessData{
		Gender: loc.T(i18n.System.Gender, nil, i18n.WithGender(cm.Gender())),
		User:   tghtml.MemberMention(loc, ch, cm),
	}), botapi.WithParseMode(botapi.ParseModeHTML))

	return err
}
