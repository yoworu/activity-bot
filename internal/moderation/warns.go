package moderation

import (
	"activity-bot/internal/cctx"
	"activity-bot/internal/i18n"
	"activity-bot/internal/utils/tghtml"
	"fmt"
	"strings"
	"time"

	"github.com/gotd/botapi"
)

func parseReason(text string) (reason string, forever bool, ok bool) {
	lines := strings.SplitN(text, "\n", 2)

	if strings.EqualFold(strings.TrimSpace(lines[0]), "навсегда") {
		if len(lines) == 2 {
			return strings.TrimSpace(lines[1]), true, true
		}

		return "", true, true
	}

	if strings.TrimSpace(text) != "" && !strings.Contains(text, "\n") {
		return "", false, false
	}

	return strings.TrimPrefix(text, "\n"), false, true
}

func (h *Handler) Warn(c *botapi.Context) error {
	moderator := cctx.MustChatMember(c)
	args := cctx.MustArgs(c)

	target, ok := args.User()
	if !ok || moderator.ID() == target.ID() {
		return nil
	}

	until, ok := args.Until()
	if !ok {
		until = time.Now().Add(time.Hour * 24 * 7)
	}

	reasonText, _ := args.Text()

	reason, forever, ok := parseReason(reasonText)
	if !ok {
		return nil
	}

	if forever {
		until = time.Time{}
	}

	ch := cctx.MustChat(c)

	warnsCount, err := h.service.Warn(
		c,
		ch,
		target,
		moderator,
		reason,
		until,
	)
	if err != nil {
		return fmt.Errorf("service warn: %w", err)
	}

	loc := cctx.MustLocalizer(c)

	_, err = c.Reply(
		moderationMessage(
			loc,
			ch,
			target,
			moderator,
			i18n.Cmd.Moderation.Actions.Warn,
			until,
			reason,
			loc.T(
				i18n.Cmd.Moderation.Templates.Warns,
				i18n.CmdModerationTemplatesWarnsData{
					Current: warnsCount,
					Max:     ch.MaxWarns,
				},
			),
		),
		botapi.WithParseMode(botapi.ParseModeHTML),
	)

	if warnsCount >= ch.MaxWarns {
		if err := h.service.Ban(c, ch.ID, target, moderator, until, reason); err != nil {
			return fmt.Errorf("service auto ban: %w", err)
		}

		if err := c.Bot.BanChatMember(
			c,
			botapi.ID(ch.ID),
			target.ID(),
			botapi.WithBanUntil(int(until.Unix())),
		); err != nil {
			return fmt.Errorf("bot auto ban: %w", err)
		}

		loc := cctx.MustLocalizer(c)

		_, err = c.Reply(
			moderationMessage(
				loc,
				ch,
				target,
				moderator,
				i18n.Cmd.Moderation.Actions.Ban,
				until,
				reason,
			),
			botapi.WithParseMode(botapi.ParseModeHTML),
		)
	}

	return err
}

func (h *Handler) Unwarn(c *botapi.Context) error {
	args := cctx.MustArgs(c)

	target, ok := args.User()
	if !ok {
		return nil
	}

	ch := cctx.MustChat(c)
	loc := cctx.MustLocalizer(c)

	count, err := h.service.Unwarn(c, ch.ID, target.ID())
	if err != nil {
		return fmt.Errorf("service unwarn: %w", err)
	}

	text := loc.T(
		i18n.Cmd.Moderation.Unwarn.Success,
		i18n.CmdModerationUnwarnSuccessData{
			User:  tghtml.MemberMention(loc, ch, target),
			Left:  count,
			Total: ch.MaxWarns,
		},
	)

	_, err = c.Reply(
		text,
		botapi.WithParseMode(botapi.ParseModeHTML),
	)

	return err
}

func (h *Handler) ClearWarns(c *botapi.Context) error {
	args := cctx.MustArgs(c)

	target, ok := args.User()
	if !ok {
		return nil
	}

	ch := cctx.MustChat(c)

	if err := h.service.ClearWarns(c, ch.ID, target.ID()); err != nil {
		return fmt.Errorf("service clear warns: %w", err)
	}

	loc := cctx.MustLocalizer(c)

	text := loc.T(
		i18n.Cmd.Moderation.ClearWarns.Success,
		i18n.CmdModerationClearWarnsSuccessData{
			User: tghtml.MemberMention(loc, ch, target),
		},
	)

	_, err := c.Reply(
		text,
		botapi.WithParseMode(botapi.ParseModeHTML),
	)

	return err
}

func (h *Handler) ShowWarns(c *botapi.Context) error {
	args := cctx.MustArgs(c)

	target, ok := args.User()
	if !ok {
		target = cctx.MustChatMember(c)
	}

	ch := cctx.MustChat(c)
	loc := cctx.MustLocalizer(c)

	warns, err := h.service.GetWarns(c, ch.ID, target.ID())
	if err != nil {
		return fmt.Errorf("get warns: %w", err)
	}

	maxWarns, err := h.service.GetMaxWarns(c, ch.ID)
	if err != nil {
		return fmt.Errorf("get max warns: %w", err)
	}

	now := time.Now()

	active := make([]Warn, 0, len(warns))
	for _, w := range warns {
		if w.ExpiresAt.IsZero() || w.ExpiresAt.After(now) {
			active = append(active, w)
		}
	}

	if len(active) == 0 {
		_, err = c.Reply(
			loc.T(
				i18n.Cmd.Moderation.ShowWarns.Empty,
				i18n.CmdModerationShowWarnsEmptyData{
					User: tghtml.MemberMention(loc, ch, target),
				},
			),
			botapi.WithParseMode(botapi.ParseModeHTML),
		)

		return err
	}

	lines := []string{
		loc.T(
			i18n.Cmd.Moderation.ShowWarns.Header,
			i18n.CmdModerationShowWarnsHeaderData{
				User:    tghtml.MemberMention(loc, ch, target),
				Current: len(active),
				Max:     maxWarns,
			},
		),
		"",
	}

	for i, w := range active {
		line := loc.T(
			i18n.Cmd.Moderation.ShowWarns.Item,
			i18n.CmdModerationShowWarnsItemData{
				Index:     i + 1,
				Moderator: tghtml.MemberMention(loc, ch, w.Moderator),
				Created:   tghtml.DefaultDateTime(w.CreatedAt),
			},
		)

		if !w.ExpiresAt.IsZero() {
			line += "\n" + loc.T(
				i18n.Cmd.Moderation.ShowWarns.Expires,
				i18n.CmdModerationShowWarnsExpiresData{
					Until: tghtml.DefaultDateTime(w.ExpiresAt),
				},
			)
		}

		if w.Reason != "" {
			line += "\n" + loc.T(
				i18n.Cmd.Moderation.ShowWarns.Reason,
				i18n.CmdModerationShowWarnsReasonData{
					Reason: w.Reason,
				},
			)
		}

		lines = append(lines, line)
	}

	_, err = c.Reply(
		strings.Join(lines, "\n"),
		botapi.WithParseMode(botapi.ParseModeHTML),
	)

	return err
}

func (h *Handler) ShowMaxWarns(c *botapi.Context) error {
	ch := cctx.MustChat(c)
	loc := cctx.MustLocalizer(c)

	_, err := c.Reply(
		loc.T(
			i18n.Cmd.Moderation.MaxWarns.Show,
			i18n.CmdModerationMaxWarnsShowData{
				Max: ch.MaxWarns,
			},
		),
	)

	return err
}

func (h *Handler) SetMaxWarns(c *botapi.Context) error {
	args := cctx.MustArgs(c)

	maxWarns, ok := args.Number()
	if !ok {
		return nil
	}

	if maxWarns <= 0 {
		return nil
	}

	ch := cctx.MustChat(c)

	if err := h.service.SetMaxWarns(c, ch.ID, int(maxWarns)); err != nil {
		return fmt.Errorf("service set max warns: %w", err)
	}

	loc := cctx.MustLocalizer(c)

	_, err := c.Reply(
		loc.T(
			i18n.Cmd.Moderation.SetMaxWarns.Set,
			i18n.CmdModerationSetMaxWarnsSetData{
				Max: maxWarns,
			},
		),
	)

	return err
}

func (h *Handler) WarnList(c *botapi.Context) error {
	ch := cctx.MustChat(c)
	loc := cctx.MustLocalizer(c)

	warns, err := h.service.GetWarnsByChat(c, ch.ID)
	if err != nil {
		return fmt.Errorf("get warns by chat: %w", err)
	}

	maxWarns, err := h.service.GetMaxWarns(c, ch.ID)
	if err != nil {
		return fmt.Errorf("get max warns: %w", err)
	}

	now := time.Now()

	active := make([]Warn, 0, len(warns))
	for _, w := range warns {
		if w.ExpiresAt.IsZero() || w.ExpiresAt.After(now) {
			active = append(active, w)
		}
	}

	if len(active) == 0 {
		_, err = c.Reply(
			loc.T(i18n.Cmd.Moderation.WarnList.Empty, nil),
		)
		return err
	}

	lines := []string{
		loc.T(
			i18n.Cmd.Moderation.WarnList.Header,
			i18n.CmdModerationWarnListHeaderData{
				Count:    len(active),
				MaxWarns: maxWarns,
			},
		),
		"",
	}

	for i, w := range active {
		line := loc.T(
			i18n.Cmd.Moderation.WarnList.Item,
			i18n.CmdModerationWarnListItemData{
				Index:     i + 1,
				User:      tghtml.MemberMention(loc, ch, w.Target),
				Moderator: tghtml.MemberMention(loc, ch, w.Moderator),
				Created:   tghtml.DefaultDateTime(w.CreatedAt),
			},
		)

		if !w.ExpiresAt.IsZero() {
			line += "\n" + loc.T(
				i18n.Cmd.Moderation.WarnList.Expires,
				i18n.CmdModerationWarnListExpiresData{
					Until: tghtml.DefaultDateTime(w.ExpiresAt),
				},
			)
		}

		if w.Reason != "" {
			line += "\n" + loc.T(
				i18n.Cmd.Moderation.WarnList.Reason,
				i18n.CmdModerationWarnListReasonData{
					Reason: w.Reason,
				},
			)
		}

		lines = append(lines, line)
	}

	_, err = c.Reply(
		strings.Join(lines, "\n"),
		botapi.WithParseMode(botapi.ParseModeHTML),
	)

	return err
}
