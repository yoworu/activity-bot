package info

import (
	"activity-bot/internal/chatmember"
	"bytes"
	"fmt"
	"sort"
	"strings"
	"text/template"
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
	{
		Name:   "winter",
		Months: []int{12, 1, 2},
	},
	{
		Name:   "spring",
		Months: []int{3, 4, 5},
	},
	{
		Name:   "summer",
		Months: []int{6, 7, 8},
	},
	{
		Name:   "autumn",
		Months: []int{9, 10, 11},
	},
}

type BirthdayRenderData struct {
	Seasons []BirthdayRenderSeason
	Header  string
	Footer  string
}

type BirthdayRenderSeason struct {
	Name    string
	Members []BirthdayMember
	Text    string
}

const birthdaysTemplate = `{{.Header}}
{{range .Seasons -}}
{{.Name}}
<blockquote expandable>{{.Text}}</blockquote>

{{end}}
{{.Footer}}`

var birthdaysTmpl = template.Must(
	template.New("birthdays").Parse(birthdaysTemplate),
)

func BuildBirthdaySeasons(members []chatmember.ChatMember) []BirthdaySeason {
	seasons := make([]BirthdaySeason, len(birthdaySeasons))

	for i, season := range birthdaySeasons {
		seasons[i].Name = season.Name
	}

	for _, member := range members {
		if member.IsLeft() || member.Birthday.IsZero() {
			continue
		}

		birthday := member.Birthday
		month := int(birthday.Month())

		seasonIndex := (month % 12) / 3

		seasons[seasonIndex].Members = append(
			seasons[seasonIndex].Members,
			BirthdayMember{
				Name:  member.Display("", false),
				Day:   birthday.Day(),
				Month: month,
			},
		)
	}

	for i := range seasons {
		sort.Slice(
			seasons[i].Members,
			func(a, b int) bool {
				if seasons[i].Members[a].Month != seasons[i].Members[b].Month {
					return seasons[i].Members[a].Month < seasons[i].Members[b].Month
				}

				return seasons[i].Members[a].Day < seasons[i].Members[b].Day
			},
		)
	}

	result := make([]BirthdaySeason, 0, 4)

	for _, season := range seasons {
		if len(season.Members) > 0 {
			result = append(result, season)
		}
	}

	return result
}

func formatBirthday(member BirthdayMember) string {
	return fmt.Sprintf(
		"%s  —  %02d.%02d",
		member.Name,
		member.Day,
		member.Month,
	)
}

func formatBirthdays(members []BirthdayMember) string {
	var buf bytes.Buffer

	for i := 0; i < len(members); i += 2 {
		left := formatBirthday(members[i])

		buf.WriteString(left)

		if i+1 < len(members) {
			right := formatBirthday(members[i+1])

			padding := 20 - len([]rune(left))

			if padding < 4 {
				padding = 4
			}

			buf.WriteString(strings.Repeat(" ", padding))
			buf.WriteString(right)
		}

		if i+2 < len(members) {
			buf.WriteByte('\n')
		}
	}

	return buf.String()
}

func RenderBirthdays(members []chatmember.ChatMember) (string, error) {
	seasons := BuildBirthdaySeasons(members)

	data := BirthdayRenderData{
		Seasons: make([]BirthdayRenderSeason, 0, len(seasons)),
		Header:  "Дни рождения участников\n",
	}

	for _, season := range seasons {
		data.Seasons = append(
			data.Seasons,
			BirthdayRenderSeason{
				Name:    season.Name,
				Members: season.Members,
				Text:    formatBirthdays(season.Members),
			},
		)
	}

	var buf bytes.Buffer

	if err := birthdaysTmpl.Execute(&buf, data); err != nil {
		return "", err
	}

	return buf.String(), nil
}
