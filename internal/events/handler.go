package events

import (
	"activity-bot/internal/application"
	"activity-bot/internal/application2"
	"activity-bot/internal/chatmember"
	"activity-bot/internal/i18n"
	"activity-bot/internal/info"
	"activity-bot/internal/summon"
	"activity-bot/internal/user"

	"github.com/gotd/botapi"
)

type Handler struct {
	bot           *botapi.Bot
	translator    *i18n.Translator
	memberService *chatmember.Service
	userRepo      user.Repository
	updater       *info.Updater

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
		applicationRepository:  applicationRepository,
		application2Repository: application2Repository,
		summonHandler:          summonHandler,
	}
}

func (h *Handler) Attach() {
	h.bot.Dispatcher().OnChannelParticipant(h.ParticipantUpdate)
	h.bot.Dispatcher().OnBotChatInviteRequester(h.BotChatInviteRequester)
}
