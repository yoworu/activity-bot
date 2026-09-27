package info

import (
	"activity-bot/internal/chatmember"
	"activity-bot/internal/roles"
	"context"
	"fmt"
	"strings"

	"github.com/gotd/botapi"
)

type Updater struct {
	repo         chatmember.Repository
	rolesRepo    roles.Repository
	targetChatID int64
}

func NewUpdater(
	repo chatmember.Repository,
	rolesRepo roles.Repository,
	targetChatID int64,
) *Updater {
	return &Updater{
		repo:         repo,
		rolesRepo:    rolesRepo,
		targetChatID: targetChatID,
	}
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
		"H4venflood",
		24,
		text,
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

	text, err := RenderApplication(len(members), 51)
	if err != nil {
		return fmt.Errorf("render application: %w", err)
	}
	err = editCaption(
		c,
		bot,
		"H4venflood",
		18,
		text,
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
		"H4venflood",
		45,
		text,
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
		"H4venflood",
		47,
		RenderBirthdays(members),
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
) error {
	_, err := bot.EditMessageCaption(
		ctx,
		botapi.Username(username),
		msgID,
		text,
		botapi.WithParseMode(botapi.ParseModeHTML),
		botapi.DisableWebPagePreview(),
		botapi.WithReplyMarkup(
			botapi.InlineKeyboard(
				botapi.InlineRow(
					botapi.InlineButtonURL("☍ Navigation", "https://t.me/H4venflood/5"),
				),
			),
		),
	)

	if err != nil && strings.Contains(err.Error(), "message is not modified") {
		return nil
	}

	return err
}
