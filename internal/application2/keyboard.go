package application2

import (
	"activity-bot/internal/roles"
	"fmt"
	"strconv"
	"strings"

	"github.com/gotd/botapi"
)

const (
	callbackRolePrefix   = "app2:role:"
	callbackRegionPrefix = "app2:region:"
	callbackRegions      = "app2:regions"
	callbackFloodYes     = "app2:flood:yes"
	callbackFloodNo      = "app2:flood:no"
	callbackAcceptPrefix = "app2:accept:"
	callbackRejectPrefix = "app2:reject:"
	callbackNew          = "app2:new"
)

func categoryKeyboard(categories []roles.Category) *botapi.InlineKeyboardMarkup {
	buttons := make([]botapi.InlineKeyboardButton, 0, len(categories))

	for _, category := range categories {
		if category.ID == 0 {
			continue
		}

		data := botapi.InlineButtonData(
			category.Name,
			fmt.Sprintf("%s%d", callbackRegionPrefix, category.ID),
		)

		if emojiID, ok := parseEmojiID(category.Emoji); ok {
			data = data.Styled(&botapi.KeyboardButtonStyle{
				Icon: &emojiID,
			})
		}

		buttons = append(buttons, data)
	}

	return keyboardRows(buttons, 2)
}

func roleKeyboard(available []roles.Role) *botapi.InlineKeyboardMarkup {
	buttons := make([]botapi.InlineKeyboardButton, 0, len(available))

	for _, role := range available {
		if role.ID == 0 {
			continue
		}

		data := botapi.InlineButtonData(
			role.Name,
			fmt.Sprintf("%s%d", callbackRolePrefix, role.ID),
		)

		if emojiID, ok := parseEmojiID(role.Emoji); ok {
			data = data.Styled(&botapi.KeyboardButtonStyle{
				Icon: &emojiID,
			})
		}

		buttons = append(buttons, data)
	}

	markup := keyboardRows(buttons, 2)
	markup.InlineKeyboard = append(
		markup.InlineKeyboard,
		[]botapi.InlineKeyboardButton{
			botapi.InlineButtonData("« Выбрать регион", callbackRegions),
		},
	)

	return markup
}

func floodsKeyboard() *botapi.InlineKeyboardMarkup {
	return botapi.InlineKeyboard(
		botapi.InlineRow(
			botapi.InlineButtonData("Да", callbackFloodYes),
			botapi.InlineButtonData("Нет", callbackFloodNo),
		),
	)
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
