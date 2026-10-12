package events

import (
	"context"
	"fmt"
	"strings"
	"time"

	"activity-bot/internal/i18n"
	"activity-bot/internal/user"
	"activity-bot/internal/utils/chatmembers"
	"activity-bot/internal/utils/participant"
	"activity-bot/internal/utils/tghtml"

	"github.com/gotd/td/constant"
	"github.com/gotd/td/tg"

	"github.com/gotd/botapi"
)

func (h *Handler) BotChatInviteRequester(
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
	app, err := h.applicationRepository.Get(
		ctx,
		chatID,
		update.UserID,
	)

	if err != nil {
		return fmt.Errorf("get application: %w", err)
	}

	if app == nil {
		return nil
	}

	err = h.bot.ApproveChatJoinRequest(
		ctx,
		botapi.ID(chatID),
		update.UserID,
	)

	if err != nil {
		return fmt.Errorf("approve join request: %w", err)
	}

	return nil
}
func (h *Handler) ParticipantUpdate(ctx context.Context, e tg.Entities, u *tg.UpdateChannelParticipant) error {
	channel, ok := e.Channels[u.ChannelID]
	if !ok {
		return nil
	}

	if !channel.Megagroup {
		return nil
	}

	var peerID constant.TDLibPeerID
	peerID.Channel(u.ChannelID)

	chatID := int64(peerID)

	if participant.IsNotInChat(u.NewParticipant) &&
		!participant.IsNotInChat(u.PrevParticipant) {
		return h.processLeft(ctx, u, chatID)
	}

	if participant.IsNotInChat(u.PrevParticipant) &&
		!participant.IsNotInChat(u.NewParticipant) {
		return h.processJoin(ctx, e, u, chatID)
	}

	prevTag := participant.Rank(u.PrevParticipant)
	newTag := participant.Rank(u.NewParticipant)

	if newTag != "" && prevTag != newTag {
		if err := h.memberService.UpdateTag(ctx, chatID, u.UserID, newTag); err != nil {
			return fmt.Errorf("participant update tag: %w", err)
		}

		if err := h.updater.UpdateRolesPost(ctx, chatID, h.bot); err != nil {
			return fmt.Errorf("participant update: %w", err)
		}
	}

	return nil
}
func (h *Handler) processJoin(ctx context.Context, e tg.Entities, u *tg.UpdateChannelParticipant, chatID int64) error {
	ue, ok := e.Users[u.UserID]
	if !ok {
		return fmt.Errorf("user %d not found in entities", u.UserID)
	}

	userDTO := user.New(u.UserID, ue.FirstName, ue.LastName, ue.Username, user.GenderUnknown, ue.Bot, time.Now())

	res, err := h.memberService.HandleJoin(
		ctx,
		chatID,
		e.Channels[u.ChannelID].Title,
		userDTO,
		participant.Rank(u.NewParticipant),
	)
	if err != nil {
		return fmt.Errorf("handler process join: %w", err)
	}

	app, err := h.applicationRepository.Get(
		ctx,
		chatID,
		u.UserID,
	)

	if err != nil {
		return fmt.Errorf("get application on join: %w", err)
	}
	loc := h.translator.Localizer(res.ChatMember.Chat.Lang)

	if app != nil {
		if err := h.memberService.UpdateTag(
			ctx,
			chatID,
			u.UserID,
			app.Role.Name,
		); err != nil {
			return fmt.Errorf("apply application role: %w", err)
		}

		if err := h.userRepo.SetBirthday(ctx, u.UserID, app.BirthDate); err != nil {
			return fmt.Errorf("set application birth date: %w", err)
		}
		if err := h.memberService.SetEmoji(ctx, chatID, u.UserID, app.Role.Emoji); err != nil {
			return fmt.Errorf("process join set emoji: %w", err)
		}
		if err := setMemberTagRetry(ctx, h.bot, chatID, u.UserID, app.Role.Name); err != nil {
			return fmt.Errorf("process join set tag: %w", err)
		}

		if err := h.updater.UpdateRolesPost(
			ctx,
			chatID,
			h.bot,
		); err != nil {
			return fmt.Errorf("update roles after application: %w", err)
		}

		cms, err := h.memberService.ListSummonChatMembers(ctx, chatID)
		if err != nil {
			return fmt.Errorf("get members for summon: %w", err)
		}

		var key i18n.MessageID
		var data any
		var opts []i18n.LocalizeOption
		role := strings.TrimSpace(app.Role.Emoji + " " + app.Role.Name)
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

		if err := h.applicationRepository.Delete(
			ctx,
			chatID,
			u.UserID,
		); err != nil {
			return fmt.Errorf("delete application: %w", err)
		}
		if err := h.updater.UpdateApplyPost(ctx, chatID, h.bot); err != nil {
			return fmt.Errorf("process join: %w", err)
		}
		return nil
	}

	if res.IsNew && participant.IsSelf(u.NewParticipant) {
		members, err := participant.GetChatMembers(h.bot, ctx, u.ChannelID)
		if err != nil {
			return fmt.Errorf("get chat members on bot join: %w", err)
		}

		if err = h.memberService.SyncChatMembers(
			ctx,
			chatID,
			chatmembers.ExtractMembers(members),
		); err != nil {
			return fmt.Errorf("sync chat members: %w", err)
		}

		var text string

		if participant.IsAdmin(u.NewParticipant) {
			text = loc.T(i18n.System.BotAddedAdmin, i18n.SystemBotAddedData{
				Emoji: tghtml.PatPatEmoji(),
			})
		} else {
			text = loc.T(i18n.System.BotAdded, i18n.SystemBotAddedAdminData{
				Emoji: tghtml.PatPatEmoji(),
			})
		}

		_, err = h.bot.SendMessage(
			ctx,
			botapi.ID(chatID),
			text,
			botapi.WithParseMode(botapi.ParseModeHTML),
		)

		return err
	}

	cm := res.ChatMember

	mention := tghtml.MemberMention(loc, cm.Chat, cm)

	var key i18n.MessageID
	var data any

	if res.IsNew {
		key = i18n.User.Joined
		data = i18n.UserJoinedData{
			User: mention,
		}
	} else {
		key = i18n.User.Returned
		data = i18n.UserReturnedData{
			User: mention,
		}
	}

	text := loc.T(
		key,
		data,
		i18n.WithGender(cm.Gender()),
	)

	_, err = h.bot.SendMessage(
		ctx,
		botapi.ID(chatID),
		text,
		botapi.WithParseMode(botapi.ParseModeHTML),
	)
	if err := h.updater.UpdateApplyPost(ctx, chatID, h.bot); err != nil {
		return fmt.Errorf("process join: %w", err)
	}
	return err
}

func (h *Handler) processLeft(ctx context.Context, u *tg.UpdateChannelParticipant, chatID int64) error {
	cm, err := h.memberService.HandleLeft(ctx, chatID, u.UserID)
	if err != nil {
		return fmt.Errorf("handler process left: %w", err)
	}

	loc := h.translator.Localizer(cm.Chat.Lang)

	memberLink := tghtml.MemberLink(loc, cm.Chat, cm)

	text := loc.T(
		i18n.User.Left,
		i18n.UserLeftData{
			User: memberLink,
		},
		i18n.WithGender(cm.Gender()),
	)

	_, err = h.bot.SendMessage(
		ctx,
		botapi.ID(chatID),
		text,
		botapi.WithParseMode(botapi.ParseModeHTML),
		botapi.DisableWebPagePreview(),
	)
	if err := h.updater.UpdateApplyPost(ctx, chatID, h.bot); err != nil {
		return fmt.Errorf("process lefy: %w", err)
	}

	if err := h.updater.UpdateRestsPost(ctx, chatID, h.bot); err != nil {
		return fmt.Errorf("update rests: %w", err)
	}

	if err := h.updater.UpdateRolesPost(ctx, chatID, h.bot); err != nil {
		return fmt.Errorf("update rests: %w", err)
	}

	if err := h.updater.UpdateBirthdaysPost(ctx, chatID, h.bot); err != nil {
		return fmt.Errorf("update birthdays: %w", err)
	}

	return err
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
