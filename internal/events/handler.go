package events

import (
	"activity-bot/internal/application"
	"activity-bot/internal/application2"
	"activity-bot/internal/chatmember"
	"activity-bot/internal/i18n"
	"activity-bot/internal/info"
	"activity-bot/internal/summon"
	"activity-bot/internal/user"
	"context"
	"errors"

	"github.com/gotd/botapi"
)

type Handler struct {
	bot           *botapi.Bot
	translator    *i18n.Translator
	memberService *chatmember.Service
	userRepo      user.Repository
	updater       *info.Updater

	application2Updater *info.Updater

	applicationRepository  *application.Repository
	application2Repository *application2.Repository
	summonHandler          *summon.Handler
}

func NewHandler(
	b *botapi.Bot,
	t *i18n.Translator,
	ms *chatmember.Service,
	userRepo user.Repository,
	roleUpdater *info.Updater,
	application2Updater *info.Updater,
	applicationRepository *application.Repository,
	application2Repository *application2.Repository,
	summonHandler *summon.Handler,
) *Handler {
	return &Handler{
		bot:                    b,
		translator:             t,
		memberService:          ms,
		userRepo:               userRepo,
		updater:                roleUpdater,
		application2Updater:    application2Updater,
		applicationRepository:  applicationRepository,
		application2Repository: application2Repository,
		summonHandler:          summonHandler,
	}
}

func (h *Handler) Attach() {
	h.bot.Dispatcher().OnChannelParticipant(h.ParticipantUpdate)
	h.bot.Dispatcher().OnBotChatInviteRequester(h.BotChatInviteRequester)
}

func (h *Handler) updateRolesPosts(ctx context.Context, chatID int64) error {
	err := h.updater.UpdateRolesPost(ctx, chatID, h.bot)

	if h.application2Updater != nil {
		err = errors.Join(
			err,
			h.application2Updater.UpdateRolesPost(ctx, chatID, h.bot),
		)
	}

	return err
}
