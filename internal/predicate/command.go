package predicate

import (
	"activity-bot/internal/cctx"
	"strings"

	"github.com/gotd/botapi"
)

// extraBotUsernames contains additional bot usernames accepted in the
// /command@username form. Telegram does not always expose every bot username
// through Self(), so the main one is duplicated here.
var extraBotUsernames = map[string]struct{}{
	"krisapp": {},
}

func Command(name string, prefixes []string, aliases []string) botapi.Predicate {
	commands := make([]string, 0, len(aliases)+1)

	commands = append(commands, normalize(name))

	for _, alias := range aliases {
		commands = append(commands, normalize(alias))
	}

	return func(c *botapi.Context) bool {
		parsed, ok := ParseMessage(c, prefixes)
		if !ok {
			return false
		}

		self := c.Bot.Self()
		if self == nil {
			return false
		}

		botUsername := strings.ToLower(self.Username)

		for _, cmd := range commands {
			cmdLenBytes, ok := matchCommandAndGetLen(parsed.TrimmedText, cmd, botUsername)
			if !ok {
				continue
			}

			args := BuildArgsMessage(
				c.Message(),
				parsed.Text,
				parsed.Entities,
				parsed.Prefix,
				parsed.LeadingSpacesRunes,
				cmdLenBytes,
			)

			c.Context = cctx.WithCommandPrefix(c.Context, parsed.Prefix)
			c.Context = cctx.WithArgsMessage(c.Context, args)

			return true
		}

		return false
	}
}

func matchCommandAndGetLen(trimmedText, command, botUsername string) (int, bool) {
	firstLine := trimmedText
	if idx := strings.Index(trimmedText, "\n"); idx != -1 {
		firstLine = trimmedText[:idx]
	}

	words := strings.Fields(firstLine)
	if len(words) == 0 {
		return 0, false
	}

	cmdWords := strings.Fields(command)
	if len(words) < len(cmdWords) {
		return 0, false
	}

	for i, cmdWord := range cmdWords {
		word := strings.ToLower(words[i])
		last := i == len(cmdWords)-1

		if last {
			if word == cmdWord {
				continue
			}

			if strings.HasPrefix(word, cmdWord+"@") {
				username := strings.TrimPrefix(word, cmdWord+"@")
				if username == botUsername {
					continue
				}

				if _, ok := extraBotUsernames[username]; ok {
					continue
				}
			}

			return 0, false
		}

		if word != cmdWord {
			return 0, false
		}
	}

	lastWordToFind := words[len(cmdWords)-1]
	lastWordIdx := strings.Index(strings.ToLower(firstLine), strings.ToLower(lastWordToFind))
	if lastWordIdx == -1 {
		return 0, false
	}

	bytesLen := lastWordIdx + len(lastWordToFind)

	return bytesLen, true
}

func normalize(s string) string {
	return strings.Join(
		strings.Fields(strings.ToLower(s)),
		" ",
	)
}

func findPrefix(text string, prefixes []string) string {
	textLower := strings.ToLower(text)

	longest := ""

	for _, prefix := range prefixes {
		if strings.HasPrefix(textLower, strings.ToLower(prefix)) &&
			len(prefix) > len(longest) {
			longest = prefix
		}
	}

	return longest
}
