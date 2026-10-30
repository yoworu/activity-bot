package application2

import (
	"activity-bot/internal/chatmember"
	"activity-bot/internal/i18n"
	"activity-bot/internal/summon"
	"activity-bot/internal/user"
	"activity-bot/internal/utils/chatmembers"
	"activity-bot/internal/utils/participant"
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/gotd/td/constant"
	"github.com/gotd/td/tg"

	"github.com/gotd/botapi"
)

type JoinHandler struct {
	bot           *botapi.Bot
	translator    *i18n.Translator
	memberService *chatmember.Service
	userRepo      user.Repository
	repository    *Repository
	summonHandler *summon.Handler
}

func NewJoinHandler(
	bot *botapi.Bot,
	translator *i18n.Translator,
	memberService *chatmember.Service,
	userRepo user.Repository,
	repository *Repository,
	summonHandler *summon.Handler,
) *JoinHandler {
	return &JoinHandler{
		bot:           bot,
		translator:    translator,
		memberService: memberService,
		userRepo:      userRepo,
		repository:    repository,
		summonHandler: summonHandler,
	}
}

func (h *JoinHandler) Attach() {
	h.bot.Dispatcher().OnChannelParticipant(h.ParticipantUpdate)
	h.bot.Dispatcher().OnBotChatInviteRequester(h.BotChatInviteRequester)
}

func (h *JoinHandler) BotChatInviteRequester(
	ctx context.Context,
	e tg.Entities,
	update *tg.UpdateBotChatInviteRequester,
) error {
	peer, ok := update.Peer.(*tg.PeerChannel)
	if !ok {
		return nil
	}

	channel, ok := e.Channels[peer.ChannelID]
	if !ok {
		return fmt.Errorf("channel not found")
	}

	var peerID constant.TDLibPeerID
	peerID.Channel(channel.ID)

	chatID := int64(peerID)

	app, err := h.repository.Get(ctx, chatID, update.UserID)
	if err != nil {
		return fmt.Errorf("get application2: %w", err)
	}
	if app == nil {
		return nil
	}

	if err := h.bot.ApproveChatJoinRequest(
		ctx,
		botapi.ID(chatID),
		update.UserID,
	); err != nil {
		return fmt.Errorf("approve join request: %w", err)
	}

	return nil
}

func (h *JoinHandler) ParticipantUpdate(
	ctx context.Context,
	e tg.Entities,
	update *tg.UpdateChannelParticipant,
) error {
	channel, ok := e.Channels[update.ChannelID]
	if !ok {
		return nil
	}

	if !channel.Megagroup {
		return nil
	}

	var peerID constant.TDLibPeerID
	peerID.Channel(update.ChannelID)

	chatID := int64(peerID)

	if participant.IsNotInChat(update.NewParticipant) &&
		!participant.IsNotInChat(update.PrevParticipant) {
		if _, err := h.memberService.HandleLeft(ctx, chatID, update.UserID); err != nil {
			return fmt.Errorf("handle left: %w", err)
		}

		return nil
	}

	if participant.IsNotInChat(update.PrevParticipant) &&
		!participant.IsNotInChat(update.NewParticipant) {
		return h.processJoin(ctx, e, update, chatID)
	}

	return nil
}

func (h *JoinHandler) processJoin(
	ctx context.Context,
	e tg.Entities,
	update *tg.UpdateChannelParticipant,
	chatID int64,
) error {
	ue, ok := e.Users[update.UserID]
	if !ok {
		return fmt.Errorf("user %d not found in entities", update.UserID)
	}

	userDTO := user.New(
		update.UserID,
		ue.FirstName,
		ue.LastName,
		ue.Username,
		user.GenderUnknown,
		ue.Bot,
		time.Now(),
	)

	res, err := h.memberService.HandleJoin(
		ctx,
		chatID,
		e.Channels[update.ChannelID].Title,
		userDTO,
		participant.Rank(update.NewParticipant),
	)
	if err != nil {
		return fmt.Errorf("handle join: %w", err)
	}

	app, err := h.repository.Get(ctx, chatID, update.UserID)
	if err != nil {
		return fmt.Errorf("get application2 on join: %w", err)
	}

	if app != nil {
		return h.processApplicationJoin(ctx, chatID, update.UserID, res, app)
	}

	if res.IsNew && participant.IsSelf(update.NewParticipant) {
		members, err := participant.GetChatMembers(h.bot, ctx, update.ChannelID)
		if err != nil {
			return fmt.Errorf("get chat members on bot join: %w", err)
		}

		if err := h.memberService.SyncChatMembers(
			ctx,
			chatID,
			chatmembers.ExtractMembers(members),
		); err != nil {
			return fmt.Errorf("sync chat members: %w", err)
		}
	}

	return nil
}

func (h *JoinHandler) processApplicationJoin(
	ctx context.Context,
	chatID int64,
	userID int64,
	res chatmember.JoinResult,
	app *Application,
) error {
	if err := h.memberService.UpdateTag(
		ctx,
		chatID,
		userID,
		app.Role.Name,
	); err != nil {
		return fmt.Errorf("apply application role: %w", err)
	}

	if err := h.memberService.SetEmoji(
		ctx,
		chatID,
		userID,
		app.Role.Emoji,
	); err != nil {
		return fmt.Errorf("process join set emoji: %w", err)
	}

	if err := setMemberTagRetry(ctx, h.bot, chatID, userID, app.Role.Name); err != nil {
		return fmt.Errorf("process join set tag: %w", err)
	}

	cms, err := h.memberService.ListSummonChatMembers(ctx, chatID)
	if err != nil {
		return fmt.Errorf("get members for summon: %w", err)
	}

	loc := h.translator.Localizer(res.ChatMember.Chat.Lang)
	role := strings.TrimSpace(app.Role.Emoji + " " + app.Role.Name)

	var (
		key  i18n.MessageID
		data any
		opts []i18n.LocalizeOption
	)

	if res.IsNew {
		key = i18n.User.Apply.Joined
		data = i18n.UserApplyJoinedData{
			User: role,
		}
	} else {
		key = i18n.User.Returned
		data = i18n.UserReturnedData{
			User: role,
		}
		opts = append(opts, i18n.WithGender(res.ChatMember.Gender()))
	}

	summonText := loc.T(key, data, opts...)

	if err := h.summonHandler.RunSummon(
		ctx,
		h.bot,
		loc,
		chatID,
		0,
		summonText,
		res.ChatMember.Chat,
		cms,
	); err != nil {
		return fmt.Errorf("run application summon: %w", err)
	}

	if err := h.repository.Delete(ctx, chatID, userID); err != nil {
		return fmt.Errorf("delete application2: %w", err)
	}

	return nil
}

func setMemberTagRetry(
	ctx context.Context,
	bot *botapi.Bot,
	chatID int64,
	userID int64,
	tag string,
) error {
	var err error

	for i := 0; i < 5; i++ {
		err = bot.SetChatMemberTag(
			ctx,
			botapi.ID(chatID),
			userID,
			tag,
		)

		if err == nil {
			return nil
		}

		if !strings.Contains(err.Error(), "user is not a member") {
			return err
		}

		time.Sleep(time.Duration(i+1) * time.Second)
	}

	return err
}
