package application

import (
	"fmt"
	"strconv"
	"strings"
	"time"

	"github.com/gotd/botapi"
)

const (
	callbackReserveUntilNop   = "app:ru:nop"
	callbackReserveUntilBack  = "app:ru:back"
	callbackReserveUntilMonth = "app:ru:m:"
	callbackReserveUntilDay   = "app:ru:d:"

	reserveUntilMonths = 12
)

type reserveUntilKind int

const (
	reserveUntilNop reserveUntilKind = iota
	reserveUntilBack
	reserveUntilMonth
	reserveUntilPick
)

type reserveUntilStep struct {
	Kind  reserveUntilKind
	Year  int
	Month int
	Day   int
}

func parseReserveUntilCallback(data string) (reserveUntilStep, error) {
	switch {
	case data == callbackReserveUntilNop:
		return reserveUntilStep{Kind: reserveUntilNop}, nil
	case data == callbackReserveUntilBack:
		return reserveUntilStep{Kind: reserveUntilBack}, nil
	case strings.HasPrefix(data, callbackReserveUntilMonth):
		year, month, err := parseYearMonth(strings.TrimPrefix(data, callbackReserveUntilMonth))
		if err != nil {
			return reserveUntilStep{}, err
		}

		return reserveUntilStep{Kind: reserveUntilMonth, Year: year, Month: month}, nil
	case strings.HasPrefix(data, callbackReserveUntilDay):
		year, month, day, err := parseYearMonthDay(strings.TrimPrefix(data, callbackReserveUntilDay))
		if err != nil {
			return reserveUntilStep{}, err
		}

		return reserveUntilStep{Kind: reserveUntilPick, Year: year, Month: month, Day: day}, nil
	}

	return reserveUntilStep{}, fmt.Errorf("unknown reserve until callback %q", data)
}

func formatReserveUntil(year, month, day int) string {
	return formatBirthDate(year, month, day)
}

func minReserveUntil(now time.Time) time.Time {
	return dateUTC(now.Year(), int(now.Month()), now.Day())
}

func maxReserveUntil(now time.Time) time.Time {
	return minReserveUntil(now).AddDate(0, reserveUntilMonths, 0)
}

func validReserveUntil(year, month, day int, now time.Time) bool {
	date := dateUTC(year, month, day)
	if date.Year() != year || int(date.Month()) != month || date.Day() != day {
		return false
	}

	minDate := minReserveUntil(now)
	maxDate := maxReserveUntil(now)

	return !date.Before(minDate) && !date.After(maxDate)
}

func dateUTC(year, month, day int) time.Time {
	return time.Date(year, time.Month(month), day, 0, 0, 0, 0, time.UTC)
}

func reserveUntilKeyboard(year, month int, now time.Time) *botapi.InlineKeyboardMarkup {
	minDate := minReserveUntil(now)
	maxDate := maxReserveUntil(now)
	current := dateUTC(year, month, 1)

	if current.Before(dateUTC(minDate.Year(), int(minDate.Month()), 1)) {
		year, month = minDate.Year(), int(minDate.Month())
		current = dateUTC(year, month, 1)
	}
	if current.After(dateUTC(maxDate.Year(), int(maxDate.Month()), 1)) {
		year, month = maxDate.Year(), int(maxDate.Month())
		current = dateUTC(year, month, 1)
	}

	first := dateUTC(year, month, 1)
	days := daysInMonth(year, month)
	pad := (int(first.Weekday()) + 6) % 7

	rows := [][]botapi.InlineKeyboardButton{
		{
			reserveNopButton("Пн"),
			reserveNopButton("Вт"),
			reserveNopButton("Ср"),
			reserveNopButton("Чт"),
			reserveNopButton("Пт"),
			reserveNopButton("Сб"),
			reserveNopButton("Вс"),
		},
	}

	row := make([]botapi.InlineKeyboardButton, 0, 7)

	for i := 0; i < pad; i++ {
		row = append(row, reserveNopButton(" "))
	}

	for day := 1; day <= days; day++ {
		if !validReserveUntil(year, month, day, now) {
			row = append(row, reserveNopButton(strconv.Itoa(day)))
		} else {
			row = append(row, botapi.InlineButtonData(
				strconv.Itoa(day),
				fmt.Sprintf("%s%d:%d:%d", callbackReserveUntilDay, year, month, day),
			))
		}

		if len(row) == 7 {
			rows = append(rows, row)
			row = nil
		}
	}

	for len(row) > 0 && len(row) < 7 {
		row = append(row, reserveNopButton(" "))
	}
	if len(row) > 0 {
		rows = append(rows, row)
	}

	var nav []botapi.InlineKeyboardButton

	prev := current.AddDate(0, -1, 0)
	if !prev.Before(dateUTC(minDate.Year(), int(minDate.Month()), 1)) {
		nav = append(nav, botapi.InlineButtonData(
			"« "+strings.ToLower(monthNames[int(prev.Month())]),
			fmt.Sprintf("%s%d:%d", callbackReserveUntilMonth, prev.Year(), int(prev.Month())),
		))
	}

	next := current.AddDate(0, 1, 0)
	if !next.After(dateUTC(maxDate.Year(), int(maxDate.Month()), 1)) {
		nav = append(nav, botapi.InlineButtonData(
			strings.ToLower(monthNames[int(next.Month())])+" »",
			fmt.Sprintf("%s%d:%d", callbackReserveUntilMonth, next.Year(), int(next.Month())),
		))
	}

	if len(nav) > 0 {
		rows = append(rows, nav)
	}

	rows = append(rows, []botapi.InlineKeyboardButton{
		botapi.InlineButtonData("« Назад", callbackReserveUntilBack),
	})

	return &botapi.InlineKeyboardMarkup{InlineKeyboard: rows}
}

func reserveNopButton(text string) botapi.InlineKeyboardButton {
	return botapi.InlineButtonData(text, callbackReserveUntilNop)
}
