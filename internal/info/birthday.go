package info

import (
	"activity-bot/internal/chatmember"
	"fmt"
	"sort"
	"strings"
)

type BirthdayMember struct {
	Name  string
	Day   int
	Month int
}
type BirthdaySeason struct {
	Name    string
	Members []BirthdayMember
}

var birthdaySeasons = [...]struct {
	Name   string
	Months []int
}{
	{Name: "𓂃 ࣪˖ ❄ winter 𓂃", Months: []int{12, 1, 2}},
	{Name: "𓂃 ࣪˖ ❀ spring 𓂃", Months: []int{3, 4, 5}},
	{Name: "𓂃 ࣪˖ ☼ summer 𓂃", Months: []int{6, 7, 8}},
	{Name: "𓂃 ࣪˖ ❧ autumn 𓂃", Months: []int{9, 10, 11}},
}

func BuildBirthdaySeasons(members []chatmember.ChatMember) []BirthdaySeason {
	seasons := make([]BirthdaySeason, len(birthdaySeasons))
	for _, member := range members {
		if member.IsLeft() || member.Birthday.IsZero() {
			continue
		}
		birthday := member.Birthday
		month := int(birthday.Month())
		seasonIndex := (month % 12) / 3
		seasons[seasonIndex].Members = append(seasons[seasonIndex].Members, BirthdayMember{Name: member.Display("", false), Day: birthday.Day(), Month: month})
		seasons[seasonIndex].Name = birthdaySeasons[seasonIndex].Name
	}
	for i := range seasons {
		sort.Slice(seasons[i].Members, func(a, b int) bool {
			if seasons[i].Members[a].Month != seasons[i].Members[b].Month {
				return seasons[i].Members[a].Month < seasons[i].Members[b].Month
			}
			return seasons[i].Members[a].Day < seasons[i].Members[b].Day
		})
	}
	result := make([]BirthdaySeason, 0, 4)
	for _, season := range seasons {
		if len(season.Members) > 0 {
			result = append(result, season)
		}
	}
	return result
}
func RenderBirthdays(members []chatmember.ChatMember) string {
	seasons := BuildBirthdaySeasons(members)
	var b strings.Builder
	b.WriteString("Дни рождения участников\n\n")
	for _, season := range seasons {
		b.WriteString(season.Name)
		b.WriteString("\n")
		b.WriteString("<blockquote expandable>")
		for i, member := range season.Members {
			b.WriteString(member.Name)
			b.WriteString(" — ")
			b.WriteString(fmt.Sprintf("%02d.%02d", member.Day, member.Month))
			if i < len(season.Members)-1 {
				b.WriteString("\n")
			}
		}
		b.WriteString("</blockquote>")
		b.WriteString("\n")
	}
	return b.String()
}
