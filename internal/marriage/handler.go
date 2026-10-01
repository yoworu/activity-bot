package marriage

import (
	"activity-bot/internal/action"
	"activity-bot/internal/cctx"
	"activity-bot/internal/chat"
	"activity-bot/internal/chatmember"
	"activity-bot/internal/command"
	"activity-bot/internal/i18n"
	"activity-bot/internal/option"
	"activity-bot/internal/permission"
	"activity-bot/internal/rule"
	"activity-bot/internal/utils/tghtml"
	"errors"
	"fmt"
	"math/rand/v2"
	"slices"
	"strconv"
	"strings"
	"time"

	"github.com/gotd/botapi"
)

const CategoryMarriage command.Category = "marriage"

const (
	callbackAccept = "marriage_accept"
	callbackReject = "marriage_reject"
)

type Handler struct {
	service       *Service
	chatService   *chat.Service
	memberService *chatmember.Service
}

func NewHandler(
	service *Service,
	chatService *chat.Service,
	memberService *chatmember.Service,
) *Handler {
	return &Handler{
		service:       service,
		chatService:   chatService,
		memberService: memberService,
	}
}

func (h *Handler) Actions() []*command.Action {
	return []*command.Action{
		action.NewCommand(
			"marry",
			h.RequestMarriage,
			i18n.Cmd.Marry.Desc,
			CategoryMarriage,
			option.WithAliases("брак запрос", "пожениться", "брак"),
			option.WithRules(rule.User()),
		),

		action.NewCommand(
			"marriage",
			h.ShowMarriage,
			i18n.Cmd.Marriage.Desc,
			CategoryMarriage,
			option.WithAliases("мой брак"),
			option.WithRules(rule.User()),
		),

		action.NewCommand(
			"divorce",
			h.Divorce,
			i18n.Cmd.Divorce.Desc,
			CategoryMarriage,
			option.WithAliases("развод"),
			option.WithRules(rule.User().Optional()),
		),
		action.NewCommand(
			"marriages",
			h.ListMarriages,
			i18n.Cmd.Marriages.Desc,
			CategoryMarriage,
			option.WithAliases("браки"),
		),
		action.NewCallbackPrefix(
			"marriageaccept",
			callbackAccept,
			h.AcceptMarriageRequest,
			CategoryMarriage,
		),

		action.NewCallbackPrefix(
			"marriagereject",
			callbackReject,
			h.RejectMarriageRequest,
			CategoryMarriage,
		),

		action.NewCommand(
			"polygamy",
			h.EnablePolygamy,
			i18n.Cmd.EnablePolygamy.Desc,
			CategoryMarriage,
			option.WithPermission(permission.StatusAdmin),
			option.WithAliases("+полигамия"),
		),

		action.NewCommand(
			"nopolygamy",
			h.DisablePolygamy,
			i18n.Cmd.DisablePolygamy.Desc,
			CategoryMarriage,
			option.WithPermission(permission.StatusAdmin),
			option.WithAliases("-полигамия"),
		),
		action.NewCommand(
			"forcemarry",
			h.ForceMarry,
			i18n.Cmd.AdminMarry.Desc,
			CategoryMarriage,
			option.WithPermission(permission.StatusAdmin),
			option.WithAliases("поженить пару", "поженить"),
			option.WithRules(rule.User().Count(2)),
		),

		action.NewCommand(
			"forcedivorce",
			h.ForceDivorce,
			"Развести двух пользователей",
			CategoryMarriage,
			option.WithPermission(permission.StatusAdmin),
			option.WithAliases("развести пару", "развести"),
			option.WithRules(rule.User().Count(2)),
		),
	}
}

func (h *Handler) RequestMarriage(c *botapi.Context) error {
	sender := cctx.MustChatMember(c)
	ch := cctx.MustChat(c)
	loc := cctx.MustLocalizer(c)

	target, ok := cctx.MustArgs(c).AnyUser()
	if !ok {
		return nil
	}

	isBotTarget := target.User.IsBot
	isCurrentBotTarget := isBotTarget && target.User.ID == c.Bot.Self().ID

	outcome, err := h.service.HandleMarriageRequest(
		c,
		ch.ID,
		sender.User.ID,
		target.User.ID,
		isBotTarget,
		ch.AllowPolygamy,
	)
	if err != nil {
		switch {
		case errors.Is(err, ErrAlreadyMarried):
			_, err := c.Reply(loc.T(i18n.Cmd.Marry.Error.AlreadyMarried, nil))
			return err

		case errors.Is(err, ErrRequestExists):
			_, err := c.Reply(loc.T(i18n.Cmd.Marry.Error.RequestExists, nil))
			return err

		default:
			return fmt.Errorf("request marriage: %w", err)
		}
	}

	senderMention := tghtml.MemberMention(loc, ch, sender)
	targetMention := tghtml.MemberMention(loc, ch, target)

	var (
		text     string
		keyboard *botapi.InlineKeyboardMarkup
	)

	switch outcome.Type {
	case OutcomeSelf:
		text = loc.T(
			i18n.Cmd.Marry.Self,
			i18n.CmdMarrySelfData{
				User: senderMention,
			},
			i18n.WithGender(sender.Gender()),
		)

	case OutcomeDirect:
		switch {
		case isCurrentBotTarget:
			text = loc.T(
				i18n.Cmd.Marry.BotSelf,
				i18n.CmdMarryBotSelfData{
					Sender: senderMention,
				},
			)

		case isBotTarget:
			text = loc.T(
				i18n.Cmd.Marry.Bot,
				i18n.CmdMarryBotData{
					Sender: senderMention,
					Target: targetMention,
				},
				i18n.WithGender(sender.Gender()),
			)

		default:
			msg := i18n.Cmd.Marry.Direct
			if ch.AllowPolygamy {
				msg = i18n.Cmd.Marry.DirectPolygamy
			}

			text = loc.T(
				msg,
				i18n.CmdMarryDirectData{
					Sender: senderMention,
					Target: targetMention,
				},
				i18n.WithGender(sender.Gender()),
			)
		}

	case OutcomeAutoAccepted:
		msg := i18n.Cmd.Marry.AutoAccepted
		if ch.AllowPolygamy {
			msg = i18n.Cmd.Marry.AutoAcceptedPolygamy
		}

		text = loc.T(
			msg,
			i18n.CmdMarryAutoAcceptedData{
				Sender: senderMention,
				Target: targetMention,
			},
		)

	case OutcomeRequestCreated:
		msg := i18n.Cmd.Marry.Request
		if ch.AllowPolygamy {
			msg = i18n.Cmd.Marry.RequestPolygamy
		}

		text = loc.T(
			msg,
			i18n.CmdMarryRequestData{
				Sender: senderMention,
				Target: targetMention,
			},
			i18n.WithGender(sender.Gender()),
		)

		keyboard = botapi.InlineKeyboard(
			botapi.InlineRow(
				botapi.InlineButtonData(
					loc.T(i18n.Cmd.Marry.Button.Accept, nil),
					fmt.Sprintf("%s:%d", callbackAccept, sender.ID()),
				),
				botapi.InlineButtonData(
					loc.T(i18n.Cmd.Marry.Button.Reject, nil),
					fmt.Sprintf("%s:%d", callbackReject, sender.ID()),
				),
			),
		)
	}

	opts := []botapi.SendOption{
		botapi.WithParseMode(botapi.ParseModeHTML),
	}

	if keyboard != nil {
		opts = append(opts, botapi.WithReplyMarkup(keyboard))
	}

	_, err = c.Reply(text, opts...)

	return err
}

func (h *Handler) ShowMarriage(c *botapi.Context) error {
	loc := cctx.MustLocalizer(c)
	ch := cctx.MustChat(c)

	cm, ok := cctx.MustArgs(c).AnyUser()
	if !ok {
		return nil
	}

	marriage, err := h.service.GetMarriage(c, ch.ID, cm.User.ID)
	if err != nil {
		_, err := c.Reply(loc.T(i18n.Cmd.Marriage.Error.Load, nil))
		return err
	}

	if marriage == nil {
		_, err := c.Reply(loc.T(i18n.Cmd.Marriage.None, nil))
		return err
	}

	userMention := tghtml.MemberMention(loc, ch, cm)

	if marriage.User1.User.ID == marriage.User2.User.ID {
		_, err := c.Reply(
			loc.T(
				i18n.Cmd.Marriage.Self,
				i18n.CmdMarriageSelfData{
					User: userMention,
				},
				i18n.WithGender(cm.Gender()),
			),
			botapi.WithParseMode(botapi.ParseModeHTML),
		)
		return err
	}

	partner := marriage.User1
	if partner.User.ID == cm.User.ID {
		partner = marriage.User2
	}

	partnerMention := tghtml.MemberMention(loc, ch, partner)

	_, err = c.Reply(
		loc.T(
			i18n.Cmd.Marriage.Active,
			i18n.CmdMarriageActiveData{
				User:    userMention,
				Partner: partnerMention,
			},
		),
		botapi.WithParseMode(botapi.ParseModeHTML),
	)

	return err
}

func (h *Handler) AcceptMarriageRequest(c *botapi.Context) error {
	cb := c.Update.CallbackQuery
	if cb == nil {
		return nil
	}

	loc := cctx.MustLocalizer(c)
	ch := cctx.MustChat(c)

	fromUserID, err := parseMarriageCallbackUserID(cb.Data)
	if err != nil {
		return err
	}

	toUserID := cb.From.ID

	if err := h.service.AcceptMarriageRequest(c, ch.ID, fromUserID, toUserID, ch.AllowPolygamy); err != nil {
		return c.AnswerCallback(
			botapi.WithCallbackText(loc.T(i18n.Cmd.Marry.Error.Accept, nil)),
		)
	}

	fromMember, err := h.memberService.Get(c, ch.ID, fromUserID)
	if err != nil {
		return err
	}

	toMember, err := h.memberService.Get(c, ch.ID, toUserID)
	if err != nil {
		return err
	}

	fromMention := tghtml.MemberMention(loc, ch, fromMember)
	toMention := tghtml.MemberMention(loc, ch, toMember)

	chatID, _ := c.Chat()

	if _, err = c.Bot.EditMessageText(
		c,
		chatID,
		cb.Message.MessageID,
		loc.T(i18n.Cmd.Marry.Accepted, nil),
		botapi.WithParseMode(botapi.ParseModeHTML),
	); err != nil {
		return err
	}

	_ = c.AnswerCallback(
		botapi.WithCallbackText(loc.T(i18n.Cmd.Marry.Callback.Accepted, nil)),
	)
	var marriageAnnounces []i18n.MessageID

	if ch.AllowPolygamy {
		marriageAnnounces = []i18n.MessageID{
			i18n.Cmd.Marry.AnnouncePolygamy1,
			i18n.Cmd.Marry.AnnouncePolygamy2,
			i18n.Cmd.Marry.AnnouncePolygamy3,
			i18n.Cmd.Marry.AnnouncePolygamy4,
			i18n.Cmd.Marry.AnnouncePolygamy5,
			i18n.Cmd.Marry.AnnouncePolygamy6,
		}
	} else {
		marriageAnnounces = []i18n.MessageID{
			i18n.Cmd.Marry.Announce1,
			i18n.Cmd.Marry.Announce2,
			i18n.Cmd.Marry.Announce3,
			i18n.Cmd.Marry.Announce4,
			i18n.Cmd.Marry.Announce5,
			i18n.Cmd.Marry.Announce6,
		}
	}

	announce := marriageAnnounces[rand.IntN(len(marriageAnnounces))]

	_, err = c.Bot.SendMessage(
		c,
		chatID,
		loc.T(
			announce,
			i18n.CmdMarryAnnounce1Data{
				Sender: fromMention,
				Target: toMention,
			},
		),
		botapi.WithParseMode(botapi.ParseModeHTML),
	)

	return err
}

func (h *Handler) RejectMarriageRequest(c *botapi.Context) error {
	cb := c.Update.CallbackQuery
	if cb == nil {
		return nil
	}

	loc := cctx.MustLocalizer(c)
	ch := cctx.MustChat(c)

	fromUserID, err := parseMarriageCallbackUserID(cb.Data)
	if err != nil {
		return err
	}

	toUserID := cb.From.ID

	if err := h.service.RejectMarriageRequest(c, ch.ID, fromUserID, toUserID, false); err != nil {
		return c.AnswerCallback(
			botapi.WithCallbackText(loc.T(i18n.Cmd.Marry.Error.Reject, nil)),
		)
	}

	chatID, _ := c.Chat()

	_, _ = c.Bot.EditMessageText(
		c,
		chatID,
		cb.Message.MessageID,
		loc.T(i18n.Cmd.Marry.Rejected, nil),
		botapi.WithParseMode(botapi.ParseModeHTML),
		botapi.WithReplyMarkup(&botapi.InlineKeyboardMarkup{}),
	)

	_ = c.AnswerCallback(
		botapi.WithCallbackText(loc.T(i18n.Cmd.Marry.Callback.Rejected, nil)),
	)

	return nil
}

func (h *Handler) Divorce(c *botapi.Context) error {
	loc := cctx.MustLocalizer(c)
	sender := cctx.MustChatMember(c)
	ch := cctx.MustChat(c)

	var partnerID int64
	if target, ok := cctx.MustArgs(c).User(); ok {
		partnerID = target.User.ID
	}

	divorced, err := h.service.Divorce(c, ch.ID, sender.User.ID, partnerID)
	if err != nil {
		switch {
		case errors.Is(err, ErrNoMarriage):
			_, err := c.Reply(loc.T(i18n.Cmd.Divorce.Error.NoMarriage, nil))
			return err

		case errors.Is(err, ErrNotYourMarriage):
			_, err := c.Reply(loc.T(i18n.Cmd.Divorce.Error.NotYourMarriage, nil))
			return err

		default:
			_, err := c.Reply(loc.T(i18n.Cmd.Divorce.Error.Divorce, nil))
			return err
		}
	}

	partner := divorced.User1
	if partner.User.ID == sender.User.ID {
		partner = divorced.User2
	}

	senderMention := tghtml.MemberMention(loc, ch, sender)

	if partner.User.ID == sender.User.ID {
		_, err := c.Reply(
			loc.T(
				i18n.Cmd.Divorce.Self,
				i18n.CmdDivorceSelfData{
					User: senderMention,
				},
			),
			botapi.WithParseMode(botapi.ParseModeHTML),
		)
		return err
	}

	partnerMention := tghtml.MemberMention(loc, ch, partner)

	msg := i18n.Cmd.Divorce.Announce
	if ch.AllowPolygamy {
		msg = i18n.Cmd.Divorce.AnnouncePolygamy
	}

	_, err = c.Reply(
		loc.T(
			msg,
			i18n.CmdDivorceAnnounceData{
				Sender:  senderMention,
				Partner: partnerMention,
			},
		),
		botapi.WithParseMode(botapi.ParseModeHTML),
	)

	return err
}

func (h *Handler) ListMarriages(c *botapi.Context) error {
	loc := cctx.MustLocalizer(c)
	ch := cctx.MustChat(c)

	marriages, err := h.service.ListMarriages(c, ch.ID)
	if err != nil {
		return fmt.Errorf("list marriages: %w", err)
	}

	if len(marriages) == 0 {
		_, err := c.Reply(loc.T(i18n.Cmd.Marriages.None, nil))
		return err
	}

	grouped := make(map[i18n.MessageID][]Marriage)
	for _, m := range marriages {
		category := marriageCategory(m.MarriedAt)
		grouped[category] = append(grouped[category], m)
	}

	order := []i18n.MessageID{
		i18n.Cmd.Marriages.Category.Newlyweds,
		i18n.Cmd.Marriages.Category.Green,
		i18n.Cmd.Marriages.Category.Calico,
		i18n.Cmd.Marriages.Category.Paper,
		i18n.Cmd.Marriages.Category.Leather,
		i18n.Cmd.Marriages.Category.Linen,
		i18n.Cmd.Marriages.Category.Wooden,
		i18n.Cmd.Marriages.Category.CastIron,
		i18n.Cmd.Marriages.Category.Copper,
		i18n.Cmd.Marriages.Category.Tin,
		i18n.Cmd.Marriages.Category.Faience,
		i18n.Cmd.Marriages.Category.Rose,
	}
	slices.Reverse(order)

	var b strings.Builder

	writeGroup := func(category i18n.MessageID, items []Marriage) {
		if len(items) == 0 {
			return
		}

		emoji := marriageCategoryEmoji(category)

		b.WriteString(emoji)
		b.WriteString(" ")
		b.WriteString(loc.T(category, nil))
		b.WriteByte('\n')
		b.WriteString("<blockquote expandable>")
		for i, m := range items {
			if i > 0 {
				b.WriteByte('\n')
			}

			b.WriteString(strconv.Itoa(i + 1))
			b.WriteString(". ")

			b.WriteString(tghtml.MemberLink(loc, ch, m.User1))
			b.WriteString(" + ")

			if m.User1.User.ID == m.User2.User.ID {
				b.WriteString(loc.T(i18n.Common.Self, nil, i18n.WithGender(m.User1.Gender())))
			} else {
				b.WriteString(tghtml.MemberLink(loc, ch, m.User2))
			}

			if !m.MarriedAt.IsZero() {
				b.WriteString(" (")
				b.WriteString(tghtml.RelativeDateTime(m.MarriedAt, time.Now()))
				b.WriteString(")")
			}
		}
		b.WriteString("</blockquote>")

		b.WriteString("\n\n")
	}

	for _, category := range order {
		writeGroup(category, grouped[category])
		delete(grouped, category)
	}

	for category, items := range grouped {
		writeGroup(category, items)
	}

	text := strings.TrimRight(b.String(), "\n")

	_, err = c.Reply(
		text,
		botapi.WithParseMode(botapi.ParseModeHTML),
		botapi.DisableWebPagePreview(),
	)

	return err
}

func marriageCategoryEmoji(cat i18n.MessageID) string {
	cats := map[i18n.MessageID]string{
		i18n.Cmd.Marriages.Category.Newlyweds: "💍",
		i18n.Cmd.Marriages.Category.Green:     "🌿",
		i18n.Cmd.Marriages.Category.Calico:    "🧵",
		i18n.Cmd.Marriages.Category.Paper:     "📜",
		i18n.Cmd.Marriages.Category.Leather:   "👜",
		i18n.Cmd.Marriages.Category.Linen:     "🪡",
		i18n.Cmd.Marriages.Category.Wooden:    "🪵",
		i18n.Cmd.Marriages.Category.CastIron:  "⚙️",
		i18n.Cmd.Marriages.Category.Copper:    "🟠",
		i18n.Cmd.Marriages.Category.Tin:       "🥫",
		i18n.Cmd.Marriages.Category.Faience:   "🏺",
		i18n.Cmd.Marriages.Category.Rose:      "🌹",
	}
	return cats[cat]
}

func (h *Handler) EnablePolygamy(c *botapi.Context) error {
	ch := cctx.MustChat(c)

	if err := h.chatService.SetPolygamyEnabled(c, ch.ID, true); err != nil {
		return fmt.Errorf("set polygamy enabled: %w", err)
	}

	loc := cctx.MustLocalizer(c)
	_, err := c.Reply(loc.T(i18n.Cmd.EnablePolygamy.Success, nil))
	return err
}

func (h *Handler) DisablePolygamy(c *botapi.Context) error {
	ch := cctx.MustChat(c)

	if err := h.chatService.SetPolygamyEnabled(c, ch.ID, false); err != nil {
		return fmt.Errorf("disable polygamy: %w", err)
	}

	loc := cctx.MustLocalizer(c)
	_, err := c.Reply(loc.T(i18n.Cmd.DisablePolygamy.Success, nil))
	return err
}

func marriageCategory(marriedAt time.Time) i18n.MessageID {
	switch {
	case marriedAt.IsZero():
		return i18n.Cmd.Marriages.Category.Unknown

	case time.Since(marriedAt) < 30*24*time.Hour:
		return i18n.Cmd.Marriages.Category.Newlyweds
	}

	years := int(time.Since(marriedAt).Hours() / 24 / 365)

	switch years {
	case 0:
		return i18n.Cmd.Marriages.Category.Green
	case 1:
		return i18n.Cmd.Marriages.Category.Calico
	case 2:
		return i18n.Cmd.Marriages.Category.Paper
	case 3:
		return i18n.Cmd.Marriages.Category.Leather
	case 4:
		return i18n.Cmd.Marriages.Category.Linen
	case 5:
		return i18n.Cmd.Marriages.Category.Wooden
	case 6:
		return i18n.Cmd.Marriages.Category.CastIron
	case 7:
		return i18n.Cmd.Marriages.Category.Copper
	case 8:
		return i18n.Cmd.Marriages.Category.Tin
	case 9:
		return i18n.Cmd.Marriages.Category.Faience
	case 10:
		return i18n.Cmd.Marriages.Category.Rose
	default:
		return i18n.Cmd.Marriages.Category.Rose
	}
}
func parseMarriageCallbackUserID(data string) (int64, error) {
	parts := strings.SplitN(data, ":", 2)
	if len(parts) != 2 {
		return 0, fmt.Errorf("invalid callback data")
	}
	userID, err := strconv.ParseInt(parts[1], 10, 64)
	if err != nil {
		return 0, err
	}
	return userID, nil
}

func (h *Handler) ForceMarry(c *botapi.Context) error {
	loc := cctx.MustLocalizer(c)
	ch := cctx.MustChat(c)

	users := cctx.MustArgs(c).Users

	if len(users) < 2 {
		_, err := c.Reply(
			loc.T(i18n.Cmd.AdminMarry.InvalidTarget, nil),
		)
		return err
	}

	user1 := users[0]
	user2 := users[1]

	_, err := h.service.MarryDirect(
		c,
		ch.ID,
		user1.User.ID,
		user2.User.ID,
		ch.AllowPolygamy,
	)

	if err != nil {
		switch {
		case errors.Is(err, ErrAlreadyMarried):
			_, err = c.Reply(
				loc.T(i18n.Cmd.AdminMarry.AlreadyMarried, nil),
			)
			return err

		default:
			_, err = c.Reply(
				loc.T(i18n.Cmd.AdminMarry.Error, nil),
			)
			return err
		}
	}

	user1Mention := tghtml.MemberMention(loc, ch, user1)
	user2Mention := tghtml.MemberMention(loc, ch, user2)

	_, err = c.Reply(
		loc.T(
			i18n.Cmd.AdminMarry.Success,
			i18n.CmdAdminMarrySuccessData{
				User1: user1Mention,
				User2: user2Mention,
			},
		),
		botapi.WithParseMode(botapi.ParseModeHTML),
	)

	return err
}

func (h *Handler) ForceDivorce(c *botapi.Context) error {
	loc := cctx.MustLocalizer(c)
	ch := cctx.MustChat(c)

	users := cctx.MustArgs(c).Users
	if len(users) < 2 {
		_, err := c.Reply(
			loc.T(i18n.Cmd.AdminDivorce.InvalidTarget, nil),
		)
		return err
	}

	user1 := users[0]
	user2 := users[1]

	marriage, err := h.service.GetMarriage(
		c,
		ch.ID,
		user1.User.ID,
	)

	if err != nil {
		_, err := c.Reply(
			loc.T(i18n.Cmd.AdminDivorce.Error, nil),
		)
		return err
	}

	if marriage == nil ||
		(marriage.User1.User.ID != user2.User.ID &&
			marriage.User2.User.ID != user2.User.ID) {
		_, err := c.Reply(
			loc.T(i18n.Cmd.AdminDivorce.NoMarriage, nil),
		)
		return err
	}

	if err := h.service.AdminDivorce(
		c,
		ch.ID,
		marriage.ID,
	); err != nil {
		_, err := c.Reply(
			loc.T(i18n.Cmd.AdminDivorce.Error, nil),
		)
		return err
	}

	user1Mention := tghtml.MemberMention(loc, ch, user1)
	user2Mention := tghtml.MemberMention(loc, ch, user2)

	_, err = c.Reply(
		loc.T(
			i18n.Cmd.AdminDivorce.Success,
			i18n.CmdAdminDivorceSuccessData{
				User1: user1Mention,
				User2: user2Mention,
			},
		),
		botapi.WithParseMode(botapi.ParseModeHTML),
	)

	return err
}
