package info

import (
	"activity-bot/internal/chatmember"
	"activity-bot/internal/roles"
	"context"
	"fmt"
	"net/url"
	"strconv"
	"strings"

	"github.com/gotd/botapi"
)

type Updater struct {
	repo         chatmember.Repository
	rolesRepo    roles.Repository
	targetChatID int64

	rolesPostUsername string
	rolesPostID       int

	rolesPostNavigationURL string
}

const defaultNavigationURL = "https://t.me/h4venflood/34"

func NewUpdater(
	repo chatmember.Repository,
	rolesRepo roles.Repository,
	targetChatID int64,
) *Updater {
	return &Updater{
		repo:                   repo,
		rolesRepo:              rolesRepo,
		targetChatID:           targetChatID,
		rolesPostUsername:      "h4venflood",
		rolesPostID:            41,
		rolesPostNavigationURL: defaultNavigationURL,
	}
}

func NewUpdaterForRolesPost(
	repo chatmember.Repository,
	rolesRepo roles.Repository,
	targetChatID int64,
	postLink string,
) (*Updater, error) {
	username, messageID, err := ParsePostLink(postLink)
	if err != nil {
		return nil, fmt.Errorf("parse roles post link: %w", err)
	}

	updater := NewUpdater(repo, rolesRepo, targetChatID)
	updater.rolesPostUsername = username
	updater.rolesPostID = messageID
	updater.rolesPostNavigationURL = ""

	return updater, nil
}

func ParsePostLink(link string) (string, int, error) {
	link = strings.TrimSpace(link)
	if link == "" {
		return "", 0, fmt.Errorf("post link is empty")
	}

	if !strings.Contains(link, "://") {
		link = "https://" + link
	}

	parsed, err := url.Parse(link)
	if err != nil {
		return "", 0, fmt.Errorf("parse url: %w", err)
	}

	parts := strings.Split(strings.Trim(parsed.Path, "/"), "/")
	if len(parts) != 2 {
		return "", 0, fmt.Errorf("invalid post link %q", link)
	}

	username := strings.TrimPrefix(parts[0], "@")
	if username == "" {
		return "", 0, fmt.Errorf("post link username is empty")
	}

	messageID, err := strconv.Atoi(parts[1])
	if err != nil || messageID <= 0 {
		return "", 0, fmt.Errorf("invalid post message id %q", parts[1])
	}

	return username, messageID, nil
}

func (r *Updater) UpdateRolesPost(c context.Context, chatID int64, bot *botapi.Bot) error {
	if r.targetChatID != chatID {
		return nil
	}
	members, err := r.repo.List(c, chatmember.Filter{
		ChatID: r.targetChatID,
		IsBot: chatmember.OptionalBool{
			Bool:  false,
			Valid: true,
		},
		Left: chatmember.OptionalBool{
			Bool:  false,
			Valid: true,
		},
	})
	if err != nil {
		return fmt.Errorf("get chat members: %w", err)
	}

	fandoms, err := r.rolesRepo.ListRoleTemplates(c, chatID)
	if err != nil {
		return fmt.Errorf("list role templates: %w", err)
	}

	reservations, err := r.rolesRepo.ListRoleReservations(c, chatID)
	if err != nil {
		return fmt.Errorf("list role reservations: %w", err)
	}

	text, err := Render(fandoms, BuildRoleStates(fandoms, members, reservations))
	if err != nil {
		return fmt.Errorf("render role states: %w", err)
	}
	err = editCaption(
		c,
		bot,
		r.rolesPostUsername,
		r.rolesPostID,
		text,
		r.rolesPostNavigationURL,
	)

	if err != nil {
		return fmt.Errorf("update roles: %w", err)
	}

	return nil
}

func (r *Updater) UpdateApplyPost(c context.Context, chatID int64, bot *botapi.Bot) error {
	if r.targetChatID != chatID {
		return nil
	}
	members, err := r.repo.List(c, chatmember.Filter{
		ChatID: r.targetChatID,
		IsBot: chatmember.OptionalBool{
			Bool:  false,
			Valid: true,
		},
		Left: chatmember.OptionalBool{
			Bool:  false,
			Valid: true,
		},
	})
	if err != nil {
		return fmt.Errorf("get chat members: %w", err)
	}

	text, err := RenderApplication(len(members), 50)
	if err != nil {
		return fmt.Errorf("render application: %w", err)
	}
	err = editCaption(
		c,
		bot,
		"H4venflood",
		51,
		text,
		defaultNavigationURL,
	)

	if err != nil {
		return fmt.Errorf("update apply: %w", err)

	}

	return nil
}

func (r *Updater) UpdateRestsPost(c context.Context, chatID int64, bot *botapi.Bot) error {
	if r.targetChatID != chatID {
		return nil
	}

	members, err := r.repo.List(c, chatmember.Filter{
		ChatID: r.targetChatID,
		IsBot: chatmember.OptionalBool{
			Bool:  false,
			Valid: true,
		},
		Left: chatmember.OptionalBool{
			Bool:  false,
			Valid: true,
		},
	})

	if err != nil {
		return fmt.Errorf("get chat members: %w", err)
	}

	text, err := RenderRests(BuildRestMembers(members))
	if err != nil {
		return fmt.Errorf("render rests: %w", err)
	}

	err = editCaption(
		c,
		bot,
		"h4venflood",
		45,
		text,
		defaultNavigationURL,
	)

	if err != nil {
		return fmt.Errorf("update rests: %w", err)
	}

	return nil
}

func (r *Updater) UpdateBirthdaysPost(c context.Context, chatID int64, bot *botapi.Bot) error {
	if r.targetChatID != chatID {
		return nil
	}

	members, err := r.repo.List(c, chatmember.Filter{
		ChatID: r.targetChatID,
		IsBot: chatmember.OptionalBool{
			Bool:  false,
			Valid: true,
		},
		Left: chatmember.OptionalBool{
			Bool:  false,
			Valid: true,
		},
	})

	if err != nil {
		return fmt.Errorf("get chat members: %w", err)
	}

	err = editCaption(
		c,
		bot,
		"h4venflood",
		47,
		RenderBirthdays(members),
		defaultNavigationURL,
	)

	if err != nil {
		return fmt.Errorf("update rests: %w", err)
	}

	return nil

}

func editCaption(
	ctx context.Context,
	bot *botapi.Bot,
	username string,
	msgID int,
	text string,
	navigationURL string,
) error {
	opts := []botapi.SendOption{
		botapi.WithParseMode(botapi.ParseModeHTML),
		botapi.DisableWebPagePreview(),
	}

	rows := make([][]botapi.InlineKeyboardButton, 0, 1)

	if navigationURL != "" {
		rows = append(rows, botapi.InlineRow(
			botapi.InlineButtonURL("☍ к навигации", navigationURL),
		))
	}

	// Пустая клавиатура явно удаляет кнопки у поста.
	opts = append(opts, botapi.WithReplyMarkup(botapi.InlineKeyboard(rows...)))

	_, err := bot.EditMessageCaption(
		ctx,
		botapi.Username(username),
		msgID,
		text,
		opts...,
	)

	if err != nil && strings.Contains(err.Error(), "message is not modified") {
		return nil
	}

	return err
}
