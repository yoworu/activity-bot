package application

import (
	"activity-bot/internal/roles"
	"fmt"
	"strconv"
	"strings"

	"github.com/gotd/botapi"
)

const (
	callbackRegionPrefix = "app:region:"
	callbackRolePrefix   = "app:role:"
	callbackRegions      = "app:regions"
)

func regionKeyboard(categories []roles.Category) *botapi.InlineKeyboardMarkup {
	buttons := make([]botapi.InlineKeyboardButton, 0, len(categories))
	for _, category := range categories {
		if category.ID == 0 {
			continue
		}

		buttons = append(buttons, botapi.InlineButtonData(
			category.Name,
			fmt.Sprintf("%s%d", callbackRegionPrefix, category.ID),
		))
	}

	return keyboardRows(buttons, 1)
}

func roleKeyboard(available []roles.Role) *botapi.InlineKeyboardMarkup {
	buttons := make([]botapi.InlineKeyboardButton, 0, len(available)+1)

	for _, role := range available {
		if role.ID == 0 {
			continue
		}

		emojiID, ok := parseEmojiID(role.Emoji)
		if !ok {
			continue
		}

		data := botapi.InlineButtonData(
			role.Name,
			fmt.Sprintf("%s%d", callbackRolePrefix, role.ID),
		)
		buttons = append(buttons, data.Styled(&botapi.KeyboardButtonStyle{
			Icon: &emojiID,
		}))
	}

	markup := keyboardRows(buttons, 2)
	markup.InlineKeyboard = append(
		markup.InlineKeyboard,
		[]botapi.InlineKeyboardButton{
			botapi.InlineButtonData(
				"« Выбрать регион",
				callbackRegions,
			),
		},
	)

	return markup
}

func parseEmojiID(s string) (int64, bool) {
	const prefix = `<tg-emoji emoji-id="`
	const suffix = `">`

	start := strings.Index(s, prefix)
	if start == -1 {
		return 0, false
	}

	start += len(prefix)

	end := strings.Index(s[start:], suffix)
	if end == -1 {
		return 0, false
	}

	end += start

	id, err := strconv.ParseInt(s[start:end], 10, 64)
	if err != nil {
		return 0, false
	}

	return id, true
}

func keyboardRows(
	buttons []botapi.InlineKeyboardButton,
	perRow int,
) *botapi.InlineKeyboardMarkup {
	if perRow < 1 {
		perRow = 1
	}

	var (
		rows [][]botapi.InlineKeyboardButton
		row  []botapi.InlineKeyboardButton
	)

	for _, button := range buttons {
		row = append(row, button)

		if len(row) == perRow {
			rows = append(rows, row)
			row = nil
		}
	}

	if len(row) > 0 {
		rows = append(rows, row)
	}

	return &botapi.InlineKeyboardMarkup{
		InlineKeyboard: rows,
	}
}
