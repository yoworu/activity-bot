package stats

import (
	"activity-bot/internal/chatmember"
	"activity-bot/internal/norm"
	"activity-bot/internal/reward"
	"activity-bot/internal/utils"
	"fmt"
	"strings"
	"time"

	"activity-bot/internal/chat"
	"activity-bot/internal/i18n"
	"activity-bot/internal/utils/tghtml"
)

func appendUserList(b *strings.Builder, loc *i18n.Localizer, users []chatmember.ChatMember) {
	var list strings.Builder

	for i, m := range users {
		list.WriteString(fmt.Sprintf(
			"%d. %s",
			i+1,
			tghtml.MemberLinkCustom(loc, false, m),
		))
		list.WriteByte('\n')
	}

	b.WriteString(tghtml.ExpandableBlockquote(list.String()))
	b.WriteByte('\n')
}

func RenderStats(loc *i18n.Localizer, ch chat.Chat, data CalculatedStats, forceSimple bool) string {
	var b strings.Builder

	b.WriteString(loc.T(
		i18n.Cmd.Stats.Title,
		i18n.CmdStatsTitleData{
			StatsEmoji: tghtml.StatsEmoji(),
			From:       tghtml.DefaultDateTime(data.FromDate),
			To:         tghtml.DefaultDateTime(data.ToDate),
		},
	))
	b.WriteString("\n\n")

	if !data.HasNorms || forceSimple {
		b.WriteString("<blockquote expandable>")

		for i, u := range data.SimpleResults {
			_, _ = fmt.Fprintf(
				&b,
				"%d. %s — %d",
				i+1,
				tghtml.MemberLink(loc, ch, u.Member),
				u.Messages,
			)

			if i != len(data.SimpleResults)-1 {
				b.WriteByte('\n')
			}
		}

		b.WriteString("</blockquote>\n\n")
	} else {
		for _, r := range data.NormResults {
			b.WriteString(loc.T(
				i18n.Cmd.Stats.NormTitle,
				i18n.CmdStatsNormTitleData{
					Name:     tghtml.Bold(utils.UcFirst(norm.LocalisedNormName(loc, r.NormName))),
					Required: tghtml.Code(fmt.Sprintf("%d", r.Required)),
				},
			))
			if len(r.Passed) == 0 && len(r.Failed) == 0 {
				b.WriteString(tghtml.Blockquote(loc.T(i18n.Cmd.Stats.EmptyList, nil)))
				b.WriteString("\n\n")
				continue
			}
			b.WriteString("\n\n")

			if len(r.Failed) > 0 {
				var failed strings.Builder

				for i, u := range r.Failed {
					failed.WriteString(loc.T(
						i18n.Cmd.Stats.UserFailed,
						i18n.CmdStatsUserFailedData{
							List:     i + 1,
							User:     tghtml.MemberLinkCustom(loc, false, u.Member),
							Messages: u.Messages,
							Required: r.Required,
						},
					))
					failed.WriteByte('\n')
				}

				b.WriteString(loc.T(
					i18n.Cmd.Stats.Failed,
					i18n.CmdStatsFailedData{
						DangerEmoji: tghtml.DangerEmoji(),
					},
				))
				b.WriteByte('\n')
				b.WriteString(tghtml.ExpandableBlockquote(failed.String()))
				b.WriteByte('\n')
			}

			if len(r.Failed) == 0 {
				b.WriteString(loc.T(
					i18n.Cmd.Stats.AllPassed,
					i18n.CmdStatsAllPassedData{
						SuccessEmoji: tghtml.SuccessEmoji(),
					},
				))
				b.WriteByte('\n')
			}

			//if len(r.Passed) > 0 {
			//	var passed strings.Builder
			//
			//	for i, u := range r.Passed {
			//		passed.WriteString(loc.T(
			//			i18n.Cmd.Stats.UserPassed,
			//			i18n.CmdStatsUserPassedData{
			//				List:     i + 1,
			//				User:     tghtml.MemberLinkCustom(loc, false, u.Member),
			//				Messages: u.Messages,
			//			},
			//		))
			//		passed.WriteByte('\n')
			//	}
			//
			//	b.WriteString(loc.T(
			//		i18n.Cmd.Stats.Passed,
			//		i18n.CmdStatsPassedData{
			//			SuccessEmoji: tghtml.SuccessEmoji(),
			//		},
			//	))
			//	b.WriteByte('\n')
			//	b.WriteString(tghtml.ExpandableBlockquote(passed.String()))
			//	b.WriteByte('\n')
			//}

			b.WriteByte('\n')
		}
	}

	if !forceSimple && data.HasNorms {
		if len(data.RestMembers) > 0 {
			b.WriteString(loc.T(
				i18n.Cmd.Stats.Resting,
				i18n.CmdStatsRestingData{
					RestEmoji: tghtml.RestEmoji(),
				},
			))
			b.WriteByte('\n')

			appendUserList(&b, loc, data.RestMembers)
			b.WriteByte('\n')
		}

		if len(data.NewbieMembers) > 0 {
			b.WriteString(loc.T(
				i18n.Cmd.Stats.Newbies,
				i18n.CmdStatsNewbiesData{
					NewbieEmoji: tghtml.NewbieEmoji(),
				},
			))
			b.WriteByte('\n')

			appendUserList(&b, loc, data.NewbieMembers)
			b.WriteByte('\n')
		}
	}

	b.WriteString(loc.T(
		i18n.Cmd.Stats.TotalMessages,
		i18n.CmdStatsTotalMessagesData{
			TotalEmoji: tghtml.TotalEmoji(),
			Total:      data.TotalMessages,
		},
	))

	return b.String()
}

func RenderProfile(
	loc *i18n.Localizer,
	ch chat.Chat,
	profile ProfileStats,
	weekStart time.Time,
	rewards []reward.Reward,
) string {
	var b strings.Builder

	now := time.Now()
	status := profile.ChatMember.Status

	b.WriteString(loc.T(
		i18n.Cmd.Profile.Title,
		i18n.CmdProfileTitleData{
			ProfileEmoji: tghtml.ProfileEmoji(),
			User:         tghtml.MemberLink(loc, ch, profile.ChatMember),
		},
	))

	b.WriteString(" · ")
	b.WriteString(tghtml.StatusEmoji(status))
	b.WriteByte(' ')
	b.WriteString(loc.T(status.TranslationKey(), nil))
	b.WriteString("\n\n")

	if profile.ChatMember.Description != "" {
		b.WriteString(
			tghtml.ExpandableBlockquote(
				tghtml.DescEmoji() + " " +
					profile.ChatMember.Description,
			),
		)
		b.WriteString("\n\n")
	}

	if profile.ChatMember.LeftAt.IsZero() {
		b.WriteString(loc.T(
			i18n.Cmd.Profile.MemberSince,
			i18n.CmdProfileMemberSinceData{
				Date: tghtml.DefaultDateTime(profile.ChatMember.JoinedAt),
				Days: tghtml.RelativeDateTime(profile.ChatMember.JoinedAt, now),
			},
		))
		b.WriteString("\n")
	} else {
		days := int(profile.ChatMember.LeftAt.Sub(profile.ChatMember.JoinedAt).Hours() / 24)

		b.WriteString(loc.T(
			i18n.Cmd.Profile.MemberPeriod,
			i18n.CmdProfileMemberPeriodData{
				From: tghtml.DefaultDateTime(profile.ChatMember.JoinedAt),
				To:   tghtml.DefaultDateTime(profile.ChatMember.LeftAt),
				Days: fmt.Sprintf("%d дн.", days),
			},
		))
		b.WriteString("\n")
	}

	restActive := profile.ChatMember.IsResting(now)

	restEndedThisWeek := !restActive &&
		!profile.ChatMember.RestUntil.IsZero() &&
		profile.ChatMember.RestUntil.After(weekStart)

	newbie := profile.ChatMember.IsNewbie(now, ch.NewbieThresholdDays)

	isExemptByRest := restActive || restEndedThisWeek

	if restActive {
		b.WriteString(loc.T(
			i18n.Cmd.Profile.RestUntil,
			i18n.CmdProfileRestUntilData{
				RestEmoji: tghtml.RestEmoji(),
				Date:      tghtml.DefaultDateTime(profile.ChatMember.RestUntil),
			},
		))
		b.WriteByte('\n')
	}

	if isExemptByRest {
		b.WriteString(loc.T(
			i18n.Cmd.Profile.RestExempt,
			i18n.CmdProfileRestExemptData{
				RestEmoji: tghtml.RestEmoji2(),
			},
		))
		b.WriteByte('\n')
	}

	if newbie && !isExemptByRest {
		b.WriteString(loc.T(
			i18n.Cmd.Profile.NewbieExempt,
			i18n.CmdProfileNewbieExemptData{
				NewbieEmoji: tghtml.NewbieEmoji(),
			},
		))
		b.WriteByte('\n')
	}

	b.WriteString("\n")
	b.WriteString(
		tghtml.ExpandableBlockquote(
			loc.T(
				i18n.Cmd.Profile.Activity,
				i18n.CmdProfileActivityData{
					CalendarEmoji: tghtml.CalendarEmoji(),
					ChartEmoji:    tghtml.RollingEmoji(),
					TotalEmoji:    tghtml.TotalEmoji(),

					Day:          tghtml.Number(profile.DayCount),
					Week:         tghtml.Number(profile.WeekCount),
					Month:        tghtml.Number(profile.MonthCount),
					DayRolling:   tghtml.Number(profile.DayRollingCount),
					WeekRolling:  tghtml.Number(profile.WeekRollingCount),
					MonthRolling: tghtml.Number(profile.MonthRollingCount),
					Total:        tghtml.Number(profile.AllTimeCount),
				},
			),
		),
	)

	if !isExemptByRest && !newbie && len(profile.Norms) > 0 {
		b.WriteString("\n\n")

		allPassed := true
		for _, n := range profile.Norms {
			if !n.Passed {
				allPassed = false
				break
			}
		}

		if allPassed {
			b.WriteString(loc.T(i18n.Cmd.Profile.AllNormsPassed, nil))
			b.WriteByte('\n')
		} else {
			for _, n := range profile.Norms {
				normName := utils.UcFirst(norm.LocalisedNormName(loc, n.Name))

				if n.Passed {
					b.WriteString(loc.T(
						i18n.Cmd.Profile.NormPassed,
						i18n.CmdProfileNormPassedData{
							SuccessEmoji: tghtml.SuccessEmoji(),
							Name:         normName,
							Required:     n.Required,
						},
					))
				} else {
					b.WriteString(loc.T(
						i18n.Cmd.Profile.NormFailed,
						i18n.CmdProfileNormFailedData{
							DangerEmoji: tghtml.DangerEmoji(),
							Name:        normName,
							Current:     n.Current,
							Required:    n.Required,
						},
					))
				}

				b.WriteByte('\n')
			}
		}
	}

	if len(rewards) > 0 {
		b.WriteString("\n\n")
		var rb strings.Builder

		limit := len(rewards)
		if limit > 3 {
			limit = 3
		}

		for i := 0; i < limit; i++ {
			rw := rewards[i]

			rb.WriteString(fmt.Sprintf(
				"%d. %s %s",
				i+1,
				tghtml.Escape(rw.Reason),
				reward.RankEmoji(rw.Rank),
			))

			if i != limit-1 {
				rb.WriteByte('\n')
			}
		}

		b.WriteString(loc.T(
			i18n.Cmd.Profile.Rewards,
			i18n.CmdProfileRewardsData{
				Rewards: rb.String(),
			},
		))
	}

	return strings.TrimSpace(b.String())
}
