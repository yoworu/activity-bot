package roles

import (
	"strings"

	"activity-bot/internal/utils/tghtml"
)

const (
	addFandomCommand = "/addfandom"
	maxMessageLength = 4000
)

// RenderFandomTemplate renders role templates as /addfandom commands,
// splitting long output into messages that fit the Bot API text limit.
func RenderFandomTemplate(fandoms []Fandom) []string {
	lines := make([]string, 0)

	for i, fandom := range fandoms {
		if i > 0 {
			lines = append(lines, "")
		}

		lines = append(lines, addFandomCommand+" "+tghtml.Escape(fandom.Name))

		for _, category := range fandom.Categories {
			lines = append(lines, "## "+withEmoji(category.Emoji, tghtml.Escape(category.Name)))

			for _, role := range category.Roles {
				lines = append(lines, "- "+withEmoji(role.Emoji, tghtml.Escape(role.Name)))
			}
		}
	}

	return splitLines(lines, maxMessageLength)
}

func withEmoji(emoji, name string) string {
	if emoji == "" {
		return name
	}

	return emoji + " " + name
}

func splitLines(lines []string, limit int) []string {
	result := make([]string, 0, 1)

	var message strings.Builder
	for _, line := range lines {
		if message.Len() > 0 && message.Len()+len(line)+1 > limit {
			result = append(result, strings.Trim(message.String(), "\n"))
			message.Reset()
		}

		if message.Len() > 0 {
			message.WriteByte('\n')
		}

		message.WriteString(line)
	}

	if message.Len() > 0 {
		result = append(result, strings.Trim(message.String(), "\n"))
	}

	return result
}
