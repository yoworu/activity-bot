package stats

import (
	"cmp"
	"context"
	"fmt"
	"slices"
	"time"

	"activity-bot/internal/chatmember"
	"activity-bot/internal/norm"
)

type ChatStat struct {
	ChatMember    chatmember.ChatMember
	MessagesCount int64
}

type CalculatedNormResult struct {
	NormID   int64
	NormName string
	Required int32
	Passed   []UserResult
	Failed   []UserResult
}

type CalculatedStats struct {
	FromDate      time.Time
	ToDate        time.Time
	TotalMessages int64
	HasNorms      bool
	SimpleResults []UserResult
	NormResults   []CalculatedNormResult
	RestMembers   []chatmember.ChatMember
	NewbieMembers []chatmember.ChatMember
}

type Service struct {
	chatMemberRepo chatmember.Repository
	normRepo       norm.Repository
	statsRepo      Repository
}

func NewService(cmr chatmember.Repository, nr norm.Repository, sr Repository) *Service {
	return &Service{
		chatMemberRepo: cmr,
		normRepo:       nr,
		statsRepo:      sr,
	}
}

func (s *Service) GetChatStats(ctx context.Context, chatID int64, fromDate, toDate time.Time, newbieDays int32) (CalculatedStats, error) {
	now := time.Now()

	chatMembers, err := s.chatMemberRepo.List(ctx, chatmember.Filter{
		ChatID: chatID,
		IsBot:  chatmember.OptionalBool{Bool: false, Valid: true},
		Left:   chatmember.OptionalBool{Bool: false, Valid: true},
	})
	if err != nil {
		return CalculatedStats{}, fmt.Errorf("service members list: %w", err)
	}

	norms, err := s.normRepo.List(ctx, chatID)
	if err != nil {
		return CalculatedStats{}, fmt.Errorf("service norms: %w", err)
	}

	slices.SortFunc(norms, func(a, b norm.Norm) int {
		return cmp.Compare(a.Value, b.Value)
	})

	chatStats, err := s.statsRepo.ChatStats(ctx, chatID, fromDate, toDate)
	if err != nil {
		return CalculatedStats{}, fmt.Errorf("service chat stats: %w", err)
	}

	statsByUserID := make(map[int64]int64)

	var totalMessages int64

	for _, stat := range chatStats {
		statsByUserID[stat.ChatMember.User.ID] = stat.MessagesCount
		totalMessages += stat.MessagesCount
	}

	res := CalculatedStats{
		FromDate:      fromDate,
		ToDate:        toDate,
		TotalMessages: totalMessages,
		HasNorms:      len(norms) > 0,
	}

	var (
		activeMembers []chatmember.ChatMember
		restMembers   []chatmember.ChatMember
		newbieMembers []chatmember.ChatMember
	)

	for _, m := range chatMembers {
		switch {
		case m.IsResting(now) || restEndedInPeriod(m, fromDate, toDate):
			restMembers = append(restMembers, m)
		case m.IsNewbie(now, newbieDays):
			newbieMembers = append(newbieMembers, m)
		default:
			activeMembers = append(activeMembers, m)
		}
	}

	res.RestMembers = restMembers
	res.NewbieMembers = newbieMembers

	for _, member := range chatMembers {
		res.SimpleResults = append(res.SimpleResults, UserResult{
			Member:   member,
			Messages: statsByUserID[member.User.ID],
		})
	}

	slices.SortFunc(res.SimpleResults, func(a, b UserResult) int {
		return cmp.Compare(b.Messages, a.Messages)
	})

	if !res.HasNorms {
		return res, nil
	}

	userNorms := make(map[int64][]norm.Norm)

	var commonNorms []norm.Norm

	for _, n := range norms {
		if len(n.UserIDs) == 0 {
			commonNorms = append(commonNorms, n)
			continue
		}

		for _, uID := range n.UserIDs {
			userNorms[uID] = append(userNorms[uID], n)
		}
	}

	normMap := make(map[int64]*CalculatedNormResult)

	for _, n := range norms {
		normMap[n.ID] = &CalculatedNormResult{
			NormID:   n.ID,
			NormName: n.Name,
			Required: n.Value,
		}
	}

	for _, member := range activeMembers {
		userID := member.User.ID
		messages := statsByUserID[userID]

		uRes := UserResult{
			Member:   member,
			Messages: messages,
		}

		assignedNorms := userNorms[userID]
		if len(assignedNorms) == 0 {
			assignedNorms = commonNorms
		}

		for _, n := range assignedNorms {
			r := normMap[n.ID]

			if messages < int64(n.Value) {
				r.Failed = append(r.Failed, uRes)
				break
			}

			r.Passed = append(r.Passed, uRes)
		}
	}

	for _, r := range normMap {
		sortResults(r.Passed)
		sortResults(r.Failed)
	}

	for _, n := range norms {
		res.NormResults = append(res.NormResults, *normMap[n.ID])
	}

	return res, nil
}

func restEndedInPeriod(m chatmember.ChatMember, fromDate, toDate time.Time) bool {
	return !m.RestUntil.IsZero() &&
		!m.RestUntil.Before(fromDate) &&
		!m.RestUntil.After(toDate)
}

func sortResults(users []UserResult) {
	slices.SortFunc(users, func(a, b UserResult) int {
		if c := cmp.Compare(b.Messages, a.Messages); c != 0 {
			return c
		}

		return cmp.Compare(a.Member.User.FirstName, b.Member.User.FirstName)
	})
}

func (s *Service) GetProfileStats(
	ctx context.Context,
	chatID,
	userID int64,
	statsRange ProfileStatsRange,
) (ProfileStats, error) {
	st, err := s.statsRepo.ProfileStats(
		ctx,
		chatID,
		userID,
		statsRange,
	)
	if err != nil {
		return ProfileStats{}, fmt.Errorf("service stats: %w", err)
	}

	norms, err := s.normRepo.List(ctx, chatID)
	if err != nil {
		return ProfileStats{}, fmt.Errorf("list norms: %w", err)
	}

	var (
		commonNorms   []norm.Norm
		personalNorms []norm.Norm
	)

	for _, n := range norms {
		if len(n.UserIDs) == 0 {
			commonNorms = append(commonNorms, n)
			continue
		}

		if n.BelongsToUser(userID) {
			personalNorms = append(personalNorms, n)
		}
	}

	usedNorms := commonNorms
	if len(personalNorms) > 0 {
		usedNorms = personalNorms
	}

	for _, n := range usedNorms {
		st.Norms = append(st.Norms, ProfileNorm{
			Name:     n.Name,
			Required: n.Value,
			Current:  st.WeekCount,
			Passed:   st.WeekCount >= int64(n.Value),
		})
	}

	return st, nil
}

func (s *Service) ListInactiveMembers(ctx context.Context, chatID int64, duration time.Duration) ([]InactiveMember, error) {
	return s.statsRepo.ListInactiveMembers(ctx, chatID, duration)
}
