package stats

import (
	"activity-bot/internal/cctx"
	"activity-bot/internal/chat"
	"activity-bot/internal/chatmember"
	"activity-bot/internal/i18n"
	"activity-bot/internal/norm"
	"activity-bot/internal/utils"
	"activity-bot/internal/utils/tghtml"
	"database/sql"
	"errors"
	"fmt"
	"strconv"
	"strings"
	"time"

	"github.com/samber/lo"

	"github.com/gotd/botapi"
)

func (h *Handler) Chat(c *botapi.Context) error {
	args := cctx.MustArgs(c)
	ch := cctx.MustChat(c)

	fromDate, toDate := ParseTimeRange(ch, args)

	calculatedData, err := h.service.GetChatStats(c, ch.ID, fromDate, toDate, ch.NewbieThresholdDays)
	if err != nil {
		return fmt.Errorf("failed to calculate stats: %w", err)
	}

	loc := cctx.MustLocalizer(c)
	htmlMessage := RenderStats(loc, ch, calculatedData, false)

	opts := []botapi.SendOption{
		botapi.WithParseMode(botapi.ParseModeHTML),
		botapi.DisableWebPagePreview(),
	}

	if calculatedData.HasNorms {
		callbackData := fmt.Sprintf("summon_no_norm:%d:%d", fromDate.Unix(), toDate.Unix())
		opts = append(opts, botapi.WithReplyMarkup(
			botapi.InlineKeyboard(botapi.InlineRow(
				botapi.InlineButtonData(loc.T(i18n.Cmd.Stats.SummonNoNorm, nil), callbackData),
			)),
		))
	}

	_, err = c.Reply(
		htmlMessage,
		opts...,
	)

	return err
}

func (h *Handler) Top(c *botapi.Context) error {
	args := cctx.MustArgs(c)
	ch := cctx.MustChat(c)

	fromDate, toDate := ParseTimeRange(ch, args)

	calculatedData, err := h.service.GetChatStats(c, ch.ID, fromDate, toDate, ch.NewbieThresholdDays)
	if err != nil {
		return fmt.Errorf("failed to calculate stats: %w", err)
	}

	loc := cctx.MustLocalizer(c)
	htmlMessage := RenderStats(loc, ch, calculatedData, true)

	_, err = c.Reply(
		htmlMessage,
		botapi.WithParseMode(botapi.ParseModeHTML),
		botapi.DisableWebPagePreview(),
	)

	return err
}

func ParseTimeRange(ch chat.Chat, args cctx.ParsedArgs) (from, to time.Time) {
	loc, _ := time.LoadLocation("Europe/Moscow")
	now := time.Now().In(loc)

	if t, ok := args.Text(); ok && isAllTime(t) {
		return time.Time{}, now
	}

	switch {
	case len(args.Durations) == 1:
		return now.Add(-args.Durations[0]), now

	case len(args.DateTimes) == 2:
		return args.DateTimes[0], args.DateTimes[1]

	default:
		return currentChatWeekRange(
			now,
			ch.WeekStartDay,
			ch.WeekStartTimeMicros,
		)
	}
}

func isAllTime(text string) bool {
	text = strings.ToLower(strings.TrimSpace(text))

	switch text {
	case
		"всё",
		"все",
		"всего",
		"весь",
		"вся",
		"за всё",
		"за все",
		"за всё время",
		"за все время",
		"вся история":
		return true
	}

	return false
}

func currentChatWeekRange(
	now time.Time,
	weekStartDay int16,
	weekStartTimeMicros int64,
) (from, to time.Time) {
	startTime := time.Duration(weekStartTimeMicros) * time.Microsecond

	hours := int(startTime / time.Hour)
	minutes := int((startTime % time.Hour) / time.Minute)
	seconds := int((startTime % time.Minute) / time.Second)

	var weekDay time.Weekday

	if weekStartDay == 7 {
		weekDay = time.Sunday
	} else {
		weekDay = time.Weekday(weekStartDay)
	}

	daysBack := (7 + int(now.Weekday()) - int(weekDay)) % 7

	start := time.Date(
		now.Year(),
		now.Month(),
		now.Day(),
		hours,
		minutes,
		seconds,
		0,
		now.Location(),
	).AddDate(0, 0, -daysBack)

	if start.After(now) {
		start = start.AddDate(0, 0, -7)
	}

	return start, now
}

func (h *Handler) AskForNormName(c *botapi.Context) error {
	cq := c.Update.CallbackQuery
	_ = c.AnswerCallback()
	if cq == nil {
		return nil
	}

	chatID, _ := c.Chat()
	_, _ = c.Bot.EditMessageReplyMarkup(c, chatID, cq.Message.MessageID, nil)

	sender := c.Sender()
	if sender == nil {
		return nil
	}
	ch := cctx.MustChat(c)
	loc := cctx.MustLocalizer(c)

	data := c.Update.CallbackQuery.Data
	parts := strings.Split(data, ":")
	if len(parts) != 3 {
		return fmt.Errorf("invalid callback data format: %s", data)
	}

	fromUnix, _ := strconv.ParseInt(parts[1], 10, 64)
	toUnix, _ := strconv.ParseInt(parts[2], 10, 64)

	fromDate := time.Unix(fromUnix, 0)
	toDate := time.Unix(toUnix, 0)

	norms, err := h.normRepo.List(c, ch.ID)
	if err != nil {
		return fmt.Errorf("list norms: %w", err)
	}
	if len(norms) == 0 {
		return nil
	}

	if len(norms) == 1 {
		err := h.statsFSM.Enter(c, StateAwaitSummonText, StateData{
			ChatID:   ch.ID,
			UserID:   sender.ID,
			NormName: norms[0].Name,
			FromDate: fromDate,
			ToDate:   toDate,
		})
		if err != nil {
			return fmt.Errorf("set stats fsm state: %w", err)
		}
		_, err = c.Bot.SendMessage(c, chatID,
			loc.T(i18n.Cmd.Stats.AskForSummonText, nil),
			botapi.WithParseMode(botapi.ParseModeHTML),
		)
		return err
	}

	if err := h.statsFSM.Enter(c, StateAwaitNorm, StateData{
		UserID:   sender.ID,
		ChatID:   ch.ID,
		FromDate: fromDate,
		ToDate:   toDate,
	}); err != nil {
		return fmt.Errorf(" set stats fsm state: %w", err)
	}

	_, err = c.Bot.SendMessage(c, chatID,
		loc.T(i18n.Cmd.Stats.AskForNormName, nil),
		botapi.WithParseMode(botapi.ParseModeHTML),
		botapi.WithReplyMarkup(normListKeyboard(loc, norms)),
	)
	return err
}

func normListKeyboard(loc *i18n.Localizer, norms []norm.Norm) *botapi.InlineKeyboardMarkup {
	var rows [][]botapi.InlineKeyboardButton
	for _, n := range norms {
		rows = append(rows, botapi.InlineRow(
			botapi.InlineButtonData(
				utils.UcFirst(norm.LocalisedNormName(loc, n.Name)), fmt.Sprintf("summon:norm:%d", n.ID),
			),
		))
	}
	return &botapi.InlineKeyboardMarkup{InlineKeyboard: rows}
}

func (h *Handler) ProcessNormNameCallback(c *botapi.Context) error {
	loc := cctx.MustLocalizer(c)

	session, ok, err := h.statsFSM.Get(c)
	if err != nil {
		return err
	}
	if !ok {
		return nil
	}

	cq := c.Update.CallbackQuery
	if cq == nil {
		return nil
	}

	var normID int64

	if _, err = fmt.Sscanf(cq.Data, "summon:norm:%d", &normID); err != nil {
		return fmt.Errorf("parse summon norm: %w", err)
	}
	chatID, _ := c.Chat()
	_, _ = c.Bot.EditMessageReplyMarkup(c, chatID, cq.Message.MessageID, nil)
	n, err := h.normRepo.GetByID(c, normID)
	if err != nil {
		_ = h.statsFSM.Clear(c)
		if !errors.Is(err, sql.ErrNoRows) {
			return fmt.Errorf("failed to load stats during norm validation: %w", err)
		}
		return c.AnswerCallback()
	}

	session.Data.NormName = n.Name
	err = h.statsFSM.Enter(c, StateAwaitSummonText, session.Data)
	if err != nil {
		return fmt.Errorf("failed to update state to await summon text: %w", err)
	}

	_, err = c.Bot.SendMessage(c, chatID,
		loc.T(i18n.Cmd.Stats.AskForSummonText, nil),
		botapi.WithParseMode(botapi.ParseModeHTML),
	)
	return err
}

func (h *Handler) ProcessSummonText(c *botapi.Context) error {
	msg := c.Message()
	if msg == nil {
		return nil
	}
	loc := cctx.MustLocalizer(c)
	ch := cctx.MustChat(c)

	session, ok, err := h.statsFSM.Get(c)
	if err != nil {
		return err
	}
	if !ok {
		return nil
	}

	sender := msg.From
	if sender == nil || sender.ID != session.Data.UserID {
		return nil
	}

	summonText := msg.OriginalTextHTML()
	if summonText == "" {
		return nil
	}

	_ = h.statsFSM.Clear(c)

	calculatedData, err := h.service.GetChatStats(c, ch.ID, session.Data.FromDate, session.Data.ToDate, ch.NewbieThresholdDays)
	if err != nil {
		return fmt.Errorf("failed to load stats during execution: %w", err)
	}

	var targetNorm *CalculatedNormResult
	for i := range calculatedData.NormResults {
		if calculatedData.NormResults[i].NormName == session.Data.NormName {
			targetNorm = &calculatedData.NormResults[i]
			break
		}
	}

	if targetNorm == nil || len(targetNorm.Failed) == 0 {
		_, err := c.Reply(
			loc.T(i18n.Cmd.Stats.NobodyToSummon, i18n.CmdStatsNobodyToSummonData{Name: norm.LocalisedNormName(loc, session.Data.NormName)}),
			botapi.WithParseMode(botapi.ParseModeHTML),
		)
		return err
	}

	var targetMembers []chatmember.ChatMember
	for _, userRes := range targetNorm.Failed {
		targetMembers = append(targetMembers, userRes.Member)
	}

	return h.summonH.Summon(
		c,
		summonText,
		msg.MessageID,
		ch,
		targetMembers,
	)
}

func (h *Handler) Cancel(c *botapi.Context) error {
	loc := cctx.MustLocalizer(c)

	if err := h.statsFSM.Clear(c); err != nil {
		return fmt.Errorf("cancel clear fsm: %w", err)
	}

	_, err := c.Reply(
		loc.T(i18n.System.Canceled, nil),
		botapi.WithParseMode(botapi.ParseModeHTML),
	)
	return err
}
func (h *Handler) ListInactive(c *botapi.Context) error {
	ch := cctx.MustChat(c)
	loc := cctx.MustLocalizer(c)
	duration, ok := cctx.MustArgs(c).Duration()
	if !ok {
		duration = 24 * time.Hour
	}

	members, err := h.service.ListInactiveMembers(c, ch.ID, duration)
	if err != nil {
		return fmt.Errorf("list inactive members: %w", err)
	}

	var b strings.Builder

	b.WriteString(loc.T(i18n.Cmd.Inactive.Title, i18n.CmdInactiveTitleData{
		InactiveEmoji: "💤",
	}))

	b.WriteString("\n\n")
	b.WriteString("<blockquote expandable>")
	if len(members) == 0 {
		b.WriteString(loc.T(i18n.Cmd.Inactive.EmptyList, nil))
	} else {
		for i, member := range members {
			lastActivity := loc.T(i18n.Cmd.Inactive.Never, nil)
			if !member.LastActivity.IsZero() {
				lastActivity = tghtml.RelativeDateTime(member.LastActivity, time.Now())
			}

			b.WriteString(loc.T(i18n.Cmd.Inactive.User, i18n.CmdInactiveUserData{
				List:         i + 1,
				User:         tghtml.MemberLink(loc, ch, member.ChatMember),
				LastActivity: lastActivity,
			}))
			if i != len(members)-1 {
				b.WriteByte('\n')
			}
		}
	}
	b.WriteString("</blockquote>")

	_, err = c.Reply(
		b.String(),
		botapi.WithParseMode(botapi.ParseModeHTML),
		botapi.DisableWebPagePreview(),
		botapi.WithReplyMarkup(
			botapi.InlineKeyboard(
				botapi.InlineRow(
					botapi.InlineButtonData(
						loc.T(i18n.Cmd.Inactive.SummonButton, nil),
						fmt.Sprintf("summon_inactive:%d", duration.Milliseconds())),
				),
			),
		),
	)

	return err
}

func (h *Handler) AskInactiveSummonText(c *botapi.Context) error {
	cq := c.Update.CallbackQuery
	if cq == nil {
		return nil
	}
	_ = c.AnswerCallback()
	sender := c.Sender()
	if sender == nil {
		return nil
	}

	ch := cctx.MustChat(c)
	loc := cctx.MustLocalizer(c)

	rawDuration := strings.TrimPrefix(cq.Data, "summon_inactive:")
	duration, err := strconv.Atoi(rawDuration)
	if err != nil {
		return err
	}

	if err := h.statsFSM.Enter(c, StateAwaitInactiveSummonText, StateData{
		UserID:   sender.ID,
		ChatID:   ch.ID,
		Duration: time.Duration(duration) * time.Millisecond,
	}); err != nil {
		return err
	}

	chatID, _ := c.Chat()

	if cq := c.Update.CallbackQuery; cq != nil {
		_, _ = c.Bot.EditMessageReplyMarkup(
			c,
			chatID,
			cq.Message.MessageID,
			botapi.InlineKeyboard(),
		)
	}

	_, err = c.Bot.SendMessage(
		c,
		chatID,
		loc.T(i18n.Cmd.Summon.EnterText, nil),
		botapi.WithParseMode(botapi.ParseModeHTML),
	)

	return err
}

func (h *Handler) ProcessInactiveSummonText(c *botapi.Context) error {
	msg := c.Message()
	if msg == nil {
		return nil
	}

	session, ok, err := h.statsFSM.Get(c)
	if err != nil {
		return err
	}
	if !ok {
		return nil
	}

	sender := msg.From
	if sender == nil || sender.ID != session.Data.UserID {
		return nil
	}

	summonText := msg.OriginalTextHTML()
	if summonText == "" {
		return nil
	}

	_ = h.statsFSM.Clear(c)

	ch := cctx.MustChat(c)
	loc := cctx.MustLocalizer(c)

	members, err := h.service.ListInactiveMembers(c, session.Data.ChatID, session.Data.Duration)
	if err != nil {
		return fmt.Errorf("list inactive members: %w", err)
	}

	if len(members) == 0 {
		_, err := c.Reply(
			loc.T(i18n.Cmd.Inactive.EmptyList, nil),
			botapi.WithParseMode(botapi.ParseModeHTML),
		)
		return err
	}

	targetMembers := lo.Map(members, func(m InactiveMember, _ int) chatmember.ChatMember {
		return m.ChatMember
	})

	return h.summonH.Summon(
		c,
		summonText,
		msg.MessageID,
		ch,
		targetMembers,
	)
}
