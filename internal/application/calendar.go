package application

import (
	"fmt"
	"strconv"
	"strings"
	"time"

	"github.com/gotd/botapi"
)

const (
	callbackBirthDatePrefix = "app:bd:"
	callbackBirthDateNop    = "app:bd:nop"
	callbackBirthDateRoles  = "app:bd:roles"
	callbackBirthDateYears  = "app:bd:yp"
	callbackBirthDateYear   = "app:bd:yr:"
	callbackBirthDateMonth  = "app:bd:m:"
	callbackBirthDateDay    = "app:bd:d:"

	birthYearSpan = 80
)

var monthNames = [...]string{
	"",
	"Январь",
	"Февраль",
	"Март",
	"Апрель",
	"Май",
	"Июнь",
	"Июль",
	"Август",
	"Сентябрь",
	"Октябрь",
	"Ноябрь",
	"Декабрь",
}

type birthDateKind int

const (
	birthDateNop birthDateKind = iota
	birthDateRoles
	birthDateYears
	birthDateMonths
	birthDateDays
	birthDatePick
)

type birthDateStep struct {
	Kind   birthDateKind
	Decade int
	Year   int
	Month  int
	Day    int
}

func minBirthYear(now time.Time) int {
	return now.Year() - birthYearSpan
}

func maxBirthYear(now time.Time) int {
	return now.Year()
}

func defaultBirthYearDecade(now time.Time) int {
	year := now.Year() - 18
	minYear := minBirthYear(now)
	maxYear := maxBirthYear(now)

	if year < minYear {
		year = minYear
	}
	if year > maxYear {
		year = maxYear
	}

	return decadeOf(year)
}

func decadeOf(year int) int {
	return year / 10 * 10
}

func parseBirthDateCallback(data string) (birthDateStep, error) {
	if data == callbackBirthDateNop {
		return birthDateStep{Kind: birthDateNop}, nil
	}
	if data == callbackBirthDateRoles {
		return birthDateStep{Kind: birthDateRoles}, nil
	}
	if data == callbackBirthDateYears {
		return birthDateStep{Kind: birthDateYears}, nil
	}

	switch {
	case strings.HasPrefix(data, callbackBirthDateYears+":"):
		decade, err := strconv.Atoi(strings.TrimPrefix(data, callbackBirthDateYears+":"))
		if err != nil {
			return birthDateStep{}, fmt.Errorf("parse decade: %w", err)
		}

		return birthDateStep{Kind: birthDateYears, Decade: decade}, nil

	case strings.HasPrefix(data, callbackBirthDateYear):
		year, err := strconv.Atoi(strings.TrimPrefix(data, callbackBirthDateYear))
		if err != nil {
			return birthDateStep{}, fmt.Errorf("parse year: %w", err)
		}

		return birthDateStep{Kind: birthDateMonths, Year: year}, nil

	case strings.HasPrefix(data, callbackBirthDateMonth):
		year, month, err := parseYearMonth(strings.TrimPrefix(data, callbackBirthDateMonth))
		if err != nil {
			return birthDateStep{}, err
		}

		return birthDateStep{Kind: birthDateDays, Year: year, Month: month}, nil

	case strings.HasPrefix(data, callbackBirthDateDay):
		year, month, day, err := parseYearMonthDay(strings.TrimPrefix(data, callbackBirthDateDay))
		if err != nil {
			return birthDateStep{}, err
		}

		return birthDateStep{Kind: birthDatePick, Year: year, Month: month, Day: day}, nil
	}

	return birthDateStep{}, fmt.Errorf("unknown birth date callback %q", data)
}

func parseYearMonth(payload string) (int, int, error) {
	parts := strings.Split(payload, ":")
	if len(parts) != 2 {
		return 0, 0, fmt.Errorf("expected year:month")
	}

	year, err := strconv.Atoi(parts[0])
	if err != nil {
		return 0, 0, fmt.Errorf("parse year: %w", err)
	}

	month, err := strconv.Atoi(parts[1])
	if err != nil {
		return 0, 0, fmt.Errorf("parse month: %w", err)
	}

	return year, month, nil
}

func parseYearMonthDay(payload string) (int, int, int, error) {
	parts := strings.Split(payload, ":")
	if len(parts) != 3 {
		return 0, 0, 0, fmt.Errorf("expected year:month:day")
	}

	year, err := strconv.Atoi(parts[0])
	if err != nil {
		return 0, 0, 0, fmt.Errorf("parse year: %w", err)
	}

	month, err := strconv.Atoi(parts[1])
	if err != nil {
		return 0, 0, 0, fmt.Errorf("parse month: %w", err)
	}

	day, err := strconv.Atoi(parts[2])
	if err != nil {
		return 0, 0, 0, fmt.Errorf("parse day: %w", err)
	}

	return year, month, day, nil
}

func formatBirthDate(year, month, day int) string {
	return fmt.Sprintf("%02d.%02d.%d", day, month, year)
}

func validBirthDate(year, month, day int, now time.Time) bool {
	if year < minBirthYear(now) || year > maxBirthYear(now) {
		return false
	}
	if month < 1 || month > 12 {
		return false
	}

	date := time.Date(year, time.Month(month), day, 0, 0, 0, 0, time.UTC)

	return date.Year() == year && int(date.Month()) == month && date.Day() == day
}

func yearKeyboard(decade int, now time.Time) *botapi.InlineKeyboardMarkup {
	minYear := minBirthYear(now)
	maxYear := maxBirthYear(now)

	if decade < decadeOf(minYear) {
		decade = decadeOf(minYear)
	}
	if decade > decadeOf(maxYear) {
		decade = decadeOf(maxYear)
	}

	buttons := make([]botapi.InlineKeyboardButton, 0, 10)

	for year := decade; year < decade+10; year++ {
		if year < minYear || year > maxYear {
			continue
		}

		buttons = append(buttons, botapi.InlineButtonData(
			strconv.Itoa(year),
			fmt.Sprintf("%s%d", callbackBirthDateYear, year),
		))
	}

	markup := keyboardRows(buttons, 4)

	var nav []botapi.InlineKeyboardButton

	prevDecade := decade - 10
	if prevDecade+9 >= minYear {
		nav = append(nav, botapi.InlineButtonData(
			fmt.Sprintf("« %d–%d", prevDecade, prevDecade+9),
			fmt.Sprintf("%s:%d", callbackBirthDateYears, prevDecade),
		))
	}

	nextDecade := decade + 10
	if nextDecade <= maxYear {
		nav = append(nav, botapi.InlineButtonData(
			fmt.Sprintf("%d–%d »", nextDecade, nextDecade+9),
			fmt.Sprintf("%s:%d", callbackBirthDateYears, nextDecade),
		))
	}

	if len(nav) > 0 {
		markup.InlineKeyboard = append(markup.InlineKeyboard, nav)
	}

	markup.InlineKeyboard = append(markup.InlineKeyboard, []botapi.InlineKeyboardButton{
		botapi.InlineButtonData("« Выбрать роль", callbackBirthDateRoles),
	})

	return markup
}

func monthKeyboard(year int) *botapi.InlineKeyboardMarkup {
	buttons := make([]botapi.InlineKeyboardButton, 0, 12)

	for month := 1; month <= 12; month++ {
		buttons = append(buttons, botapi.InlineButtonData(
			monthNames[month],
			fmt.Sprintf("%s%d:%d", callbackBirthDateMonth, year, month),
		))
	}

	markup := keyboardRows(buttons, 3)
	markup.InlineKeyboard = append(markup.InlineKeyboard, []botapi.InlineKeyboardButton{
		botapi.InlineButtonData(
			"« Назад",
			fmt.Sprintf("%s:%d", callbackBirthDateYears, decadeOf(year)),
		),
	})

	return markup
}

func dayKeyboard(year, month int) *botapi.InlineKeyboardMarkup {
	first := time.Date(year, time.Month(month), 1, 0, 0, 0, 0, time.UTC)
	days := daysInMonth(year, month)
	pad := (int(first.Weekday()) + 6) % 7

	rows := [][]botapi.InlineKeyboardButton{
		{
			nopButton("Пн"),
			nopButton("Вт"),
			nopButton("Ср"),
			nopButton("Чт"),
			nopButton("Пт"),
			nopButton("Сб"),
			nopButton("Вс"),
		},
	}

	row := make([]botapi.InlineKeyboardButton, 0, 7)

	for i := 0; i < pad; i++ {
		row = append(row, nopButton(" "))
	}

	for day := 1; day <= days; day++ {
		row = append(row, botapi.InlineButtonData(
			strconv.Itoa(day),
			fmt.Sprintf("%s%d:%d:%d", callbackBirthDateDay, year, month, day),
		))

		if len(row) == 7 {
			rows = append(rows, row)
			row = nil
		}
	}

	for len(row) > 0 && len(row) < 7 {
		row = append(row, nopButton(" "))
	}

	if len(row) > 0 {
		rows = append(rows, row)
	}

	rows = append(rows, []botapi.InlineKeyboardButton{
		botapi.InlineButtonData(
			"« Назад",
			fmt.Sprintf("%s%d", callbackBirthDateYear, year),
		),
	})

	return &botapi.InlineKeyboardMarkup{InlineKeyboard: rows}
}

func nopButton(text string) botapi.InlineKeyboardButton {
	return botapi.InlineButtonData(text, callbackBirthDateNop)
}

func daysInMonth(year, month int) int {
	return time.Date(year, time.Month(month)+1, 0, 0, 0, 0, 0, time.UTC).Day()
}
