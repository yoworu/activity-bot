package handler

import (
	"activity-bot/internal/action"
	"activity-bot/internal/cctx"
	"activity-bot/internal/chatmember"
	"activity-bot/internal/command"
	"activity-bot/internal/emoji"
	"activity-bot/internal/i18n"
	"activity-bot/internal/option"
	"activity-bot/internal/rp"
	"activity-bot/internal/rule"
	"activity-bot/internal/user"
	"activity-bot/internal/utils/tghtml"
	"errors"
	"fmt"
	"html"
	"regexp"
	"strings"

	fsm "github.com/fluffur/botapi-fsm"

	"github.com/gotd/botapi"
)

const CategoryRP = "rp"

type Handler struct {
	repo              rp.Repository
	chatMemberService *chatmember.Service
	userRepo          user.Repository
	fsm               *fsm.Machine[rp.State, rp.StateData]
}

func NewHandler(
	repo rp.Repository,
	chatMemberService *chatmember.Service,
	userRepo user.Repository,
	rpFSM *fsm.Machine[rp.State, rp.StateData],
) *Handler {
	return &Handler{
		repo:              repo,
		userRepo:          userRepo,
		chatMemberService: chatMemberService,
		fsm:               rpFSM,
	}
}

func (h *Handler) Actions() []*command.Action {
	return []*command.Action{
		action.NewCommand(
			"addrp",
			h.AddRPCommand,
			i18n.Cmd.AddRp.Desc,
			CategoryRP,
			option.WithAliases("рп", "+рп"),
			option.WithRules(rule.Text()),
			option.WithExamples(i18n.Cmd.AddRp.Example1, i18n.Cmd.AddRp.Example2),
		),
		action.NewRPCommand(
			"rp",
			h.HandleRPCommand,
			CategoryRP,
			option.WithRules(rule.User(), rule.Text().Optional()),
		),
		action.NewCommand(
			"listrp",
			h.ListRPCommand,
			i18n.Cmd.ListRp.Desc,
			CategoryRP,
			option.WithAliases("рп"),
		),
		action.NewCommand(
			"delrp",
			h.DeleteRPCommand,
			i18n.Cmd.DelRp.Desc,
			CategoryRP,
			option.WithRules(rule.Text()),
			option.WithAliases("-рп"),
		),
		action.NewCallbackPrefix(
			"rpgender",
			"rp_gender:",
			h.HandleGenderCallback,
			CategoryRP,
		),
	}
}

var ErrInvalidFormat = errors.New("invalid format")

func ParseRP(html string) (rp.Definition, error) {
	parts := strings.SplitN(strings.TrimSpace(html), "\n", 2)
	if len(parts) != 2 {
		return rp.Definition{}, ErrInvalidFormat
	}

	trigger := strings.TrimSpace(parts[0])

	actionHTML := strings.TrimSpace(parts[1])

	emojis := emoji.Extract(actionHTML)

	act := actionHTML
	for _, e := range emojis {
		act = strings.ReplaceAll(act, e, "")
	}

	act = strings.TrimSpace(act)

	return rp.Definition{
		Trigger: trigger,
		Action:  act,
		Emojis:  strings.Join(emojis, ""),
	}, nil
}

func (h *Handler) AddRPCommand(c *botapi.Context) error {
	message := cctx.MustArgsMessage(c)

	cmd, err := ParseRP(message.OriginalTextHTML())
	if err != nil {
		return fmt.Errorf("add rp parse: %w", err)
	}

	ch := cctx.MustChat(c)
	cm := cctx.MustChatMember(c)
	cmd.ChatID = ch.ID
	cmd.CreatedBy = cm.ID()

	if err := h.repo.Upsert(c, cmd); err != nil {
		return fmt.Errorf("add rp upsert: %w", err)
	}

	_, err = c.Reply(
		"Команда успешно добавлена",
		botapi.WithParseMode(botapi.ParseModeHTML),
	)
	return err
}

var parenthesisRegex = regexp.MustCompile(`([а-яА-ЯёЁ\w]+)\(([^)]+)\)`)

var genderRegex = regexp.MustCompile(`([^\s|]+)\|([^\s|]+)`)

func Genderize(s string, gender user.Gender) string {
	s = parenthesisRegex.ReplaceAllStringFunc(s, func(match string) string {
		submatches := parenthesisRegex.FindStringSubmatch(match)
		if len(submatches) < 3 {
			return match
		}
		stem := submatches[1]
		ending := submatches[2]

		if gender == user.GenderFemale {
			return stem + ending
		}
		return stem
	})

	s = genderRegex.ReplaceAllStringFunc(s, func(match string) string {
		parts := strings.SplitN(match, "|", 2)
		if len(parts) != 2 {
			return match
		}

		if gender == user.GenderFemale {
			return parts[1]
		}
		return parts[0]
	})

	return s
}
func (h *Handler) HandleRPCommand(c *botapi.Context) error {
	rpCmd := cctx.MustRPCommand(c)
	parsed := cctx.MustArgs(c)
	var extra, speech string
	if len(parsed.Texts) > 0 {
		text := parsed.Texts[0]

		parts := strings.SplitN(text, "\n", 2)

		extra = strings.TrimSpace(parts[0])

		if len(parts) == 2 {
			speech = strings.TrimSpace(parts[1])
		}
	}

	sender := cctx.MustChatMember(c)
	target, ok := cctx.MustArgs(c).AnyUser()
	if !ok {
		return nil
	}

	if user.GenderUnknown == sender.Gender() {
		return h.AskGender(c, rpCmd, sender, target, extra, speech)
	}
	msg := c.Message()
	if msg == nil {
		return nil
	}
	return h.SendRP(c, rpCmd, sender, target, extra, speech, msg.MessageID)
}

func (h *Handler) AskGender(
	c *botapi.Context,
	rpCmd rp.Definition,
	sender, target chatmember.ChatMember,
	extra, speech string,
) error {
	msg := c.Message()
	if msg == nil {
		return fmt.Errorf("ask gender no message")
	}

	err := h.fsm.Enter(
		c,
		rp.StateAwaitGender,
		rp.StateData{
			UserID:    sender.ID(),
			TargetID:  target.ID(),
			MessageID: msg.MessageID,
			CommandID: rpCmd.ID,
			Extra:     extra,
			Speech:    speech,
		},
	)
	if err != nil {
		return err
	}

	loc := cctx.MustLocalizer(c)

	_, err = c.Reply(
		loc.T(i18n.Cmd.Rp.ChooseGender, nil),
		botapi.WithReplyMarkup(
			&botapi.InlineKeyboardMarkup{
				InlineKeyboard: [][]botapi.InlineKeyboardButton{
					{
						{
							Text:         loc.T(i18n.Cmd.Rp.GenderMale, nil),
							CallbackData: "rp_gender:male",
						},
						{
							Text:         loc.T(i18n.Cmd.Rp.GenderFemale, nil),
							CallbackData: "rp_gender:female",
						},
					},
				},
			},
		),
	)

	return err
}

func (h *Handler) HandleGenderCallback(c *botapi.Context) error {
	cq := c.Update.CallbackQuery
	if cq == nil {
		return nil
	}

	chatID, _ := c.Chat()
	_ = c.Bot.DeleteMessage(c, chatID, cq.Message.MessageID)

	var gender user.Gender

	switch cq.Data {
	case "rp_gender:male":
		gender = user.GenderMale

	case "rp_gender:female":
		gender = user.GenderFemale

	default:
		return c.AnswerCallback()
	}

	state, ok, err := h.fsm.Get(c)
	if !ok {
		return c.AnswerCallback()
	}
	if err != nil {
		return err
	}

	data := state.Data

	if data.UserID != cctx.MustChatMember(c).ID() {
		return c.AnswerCallback()
	}

	rpCmd, err := h.repo.GetByID(c, data.CommandID)
	if err != nil {
		return err
	}

	sender := cctx.MustChatMember(c)

	sender.User.Gender = gender

	if err := h.userRepo.SetGender(c, sender.ID(), gender); err != nil {
		return fmt.Errorf("handle gender callback set gender: %w", err)
	}

	ch := cctx.MustChat(c)
	target, err := h.chatMemberService.Get(c, ch.ID, data.TargetID)
	if err != nil {
		return err
	}

	_ = h.fsm.Clear(c)

	_ = c.AnswerCallback()
	return h.SendRP(
		c,
		rpCmd,
		sender,
		target,
		data.Extra,
		data.Speech,
		data.MessageID,
	)
}

func (h *Handler) SendRP(
	c *botapi.Context,
	rpCmd rp.Definition,
	sender, target chatmember.ChatMember,
	extra, speech string,
	messageID int,
) error {
	act := Genderize(rpCmd.Action, sender.Gender())
	ch := cctx.MustChat(c)
	loc := cctx.MustLocalizer(c)

	var b strings.Builder

	if rpCmd.Emojis != "" {
		b.WriteString(rpCmd.Emojis)
		b.WriteString(" | ")
	}
	b.WriteString(tghtml.MemberMention(loc, ch, sender))
	b.WriteByte(' ')
	b.WriteString(act)
	b.WriteByte(' ')
	b.WriteString(tghtml.MemberMention(loc, ch, target))

	if extra != "" {
		b.WriteByte(' ')
		b.WriteString(extra)
	}

	if speech != "" {
		b.WriteByte('\n')
		b.WriteString(loc.T(i18n.Cmd.Rp.Speech, i18n.CmdRpSpeechData{
			Text: html.EscapeString(speech),
		}))
	}

	chatID, _ := c.Chat()
	_, err := c.Bot.SendMessage(
		c,
		chatID,
		b.String(),
		botapi.WithParseMode(botapi.ParseModeHTML),
		botapi.ReplyTo(messageID),
	)

	return err
}

func (h *Handler) ListRPCommand(c *botapi.Context) error {
	ch := cctx.MustChat(c)
	loc := cctx.MustLocalizer(c)
	list, err := h.repo.List(c, ch.ID)
	if err != nil {
		return fmt.Errorf("list rp: %w", err)
	}

	if len(list) == 0 {
		_, err := c.Reply(loc.T(i18n.Cmd.ListRp.Empty, nil))
		return err
	}

	var b strings.Builder
	b.WriteString(loc.T(i18n.Cmd.ListRp.Title, nil))
	b.WriteString("\n<blockquote expandable>")
	for i, item := range list {
		b.WriteString(fmt.Sprintf("• <code>%s</code>", item.Trigger))
		if i != len(list)-1 {
			b.WriteString("\n")
		}
	}
	b.WriteString("</blockquote>")

	_, err = c.Reply(b.String(), botapi.WithParseMode(botapi.ParseModeHTML))
	return err
}

func (h *Handler) DeleteRPCommand(c *botapi.Context) error {
	ch := cctx.MustChat(c)
	parsed := cctx.MustArgs(c)

	trigger, ok := parsed.Text()
	if !ok {
		return nil
	}

	if err := h.repo.Delete(c, ch.ID, trigger); err != nil {
		return fmt.Errorf("delete rp: %w", err)
	}
	loc := cctx.MustLocalizer(c)
	_, err := c.Reply(loc.T(i18n.Cmd.DelRp.Success, i18n.CmdDelRpSuccessData{
		Trigger: trigger,
	}), botapi.WithParseMode(botapi.ParseModeHTML))
	return err
}
