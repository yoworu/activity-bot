package middleware

import (
	"activity-bot/internal/cctx"
	"activity-bot/internal/chat"
	"activity-bot/internal/chatmember"
	"activity-bot/internal/i18n"
	"activity-bot/internal/message"
	"activity-bot/internal/permission"
	"activity-bot/internal/pmsession"
	"activity-bot/internal/user"
	"context"
	"database/sql"
	"errors"
	"fmt"
	"time"

	"github.com/gotd/log"
	"github.com/jackc/pgx/v5"

	"github.com/gotd/botapi"
)

func IgnoreChannelsMiddleware() botapi.Middleware {
	return func(next botapi.Handler) botapi.Handler {
		return func(c *botapi.Context) error {
			msg := c.Message()

			if msg == nil && c.Update.CallbackQuery != nil {
				msg = c.Update.CallbackQuery.Message
			}

			if msg == nil {
				return next(c)
			}

			if msg.Chat.Type == botapi.ChatTypeChannel {
				return nil
			}

			return next(c)
		}
	}
}

func ChatMiddleware(cr chat.Repository, sr pmsession.Repository) botapi.Middleware {
	return func(next botapi.Handler) botapi.Handler {
		return func(c *botapi.Context) error {
			msg := c.Message()
			if msg == nil {
				msg = c.Update.CallbackQuery.Message
			}

			if msg == nil {
				return next(c)
			}

			chatModel := chat.New(0, "")

			if msg.Chat.Type != botapi.ChatTypeGroup && msg.Chat.Type != botapi.ChatTypeSupergroup {
				ch, err := sr.GetChat(c, msg.Chat.ID)
				if err != nil {
					if errors.Is(err, sql.ErrNoRows) {
						c.Context = cctx.WithChat(c.Context, chatModel)

						return next(c)
					}

					return fmt.Errorf("get chat: %w", err)
				}

				chatModel = ch
			} else {
				ch, err := getOrCreateChat(c, cr, msg.Chat)
				if err != nil {
					return err
				}

				chatModel = ch
			}

			c.Context = cctx.WithChat(c.Context, chatModel)

			return next(c)
		}
	}
}

type UsernameChangedNotifier interface {
	NotifyUsernameChanged(c *botapi.Context, oldUsername, newUsername string) error
}

func ChatMemberMiddleware(
	cmr chatmember.Repository,
	notifier UsernameChangedNotifier,
) botapi.Middleware {
	return func(next botapi.Handler) botapi.Handler {
		return func(c *botapi.Context) error {
			msg := c.Message()
			if msg == nil && c.Update.CallbackQuery != nil {
				msg = c.Update.CallbackQuery.Message
			}

			if msg == nil {
				return next(c)
			}

			userModel := cctx.MustUser(c)

			chatModel, err := cctx.Chat(c)
			if err != nil {
				return fmt.Errorf("chat member middleware get chat: %w", err)
			}

			if chatModel.ID == 0 {
				return next(c)
			}

			member, err := getOrCreateChatMember(
				c,
				cmr,
				c.Bot,
				chatModel,
				userModel,
			)
			if err != nil {
				return fmt.Errorf("chat member middleware get chat member: %w", err)
			}

			c.Context = cctx.WithChatMember(c.Context, member)

			update := cctx.MustUserUpdate(c)
			if update.NewUsername != "" {
				if err := notifier.NotifyUsernameChanged(
					c,
					update.OldUsername,
					update.NewUsername,
				); err != nil {
					log.For(c.Bot.Logger()).Error(
						c,
						"notify username changed",
						log.Error(err),
					)
				}
			}

			return next(c)
		}
	}
}

func SaveMessageMiddleware(messageRepository message.Repository) botapi.Middleware {
	return func(next botapi.Handler) botapi.Handler {
		return func(c *botapi.Context) error {
			sender := c.Sender()
			msg := c.Message()

			if msg == nil || sender == nil {
				return next(c)
			}

			if msg.Chat.Type == botapi.ChatTypePrivate {
				return nil
			}

			if msg.From == nil || msg.From.IsBot {
				return nil
			}

			if err := messageRepository.Save(
				c,
				msg.Chat.ID,
				sender.ID,
				int64(msg.MessageID),
			); err != nil {
				return fmt.Errorf("save message: %w", err)
			}

			return next(c)
		}
	}
}

func getOrCreateChat(ctx context.Context, repo chat.Repository, ch botapi.Chat) (chat.Chat, error) {
	id := ch.ID

	model, err := repo.Get(ctx, id)
	if err == nil {
		return model, nil
	}

	if !errors.Is(err, pgx.ErrNoRows) {
		return chat.Chat{}, fmt.Errorf("get chat: %w", err)
	}

	model = chat.New(id, ch.Title)

	if err := repo.Create(ctx, model); err != nil {
		return chat.Chat{}, fmt.Errorf("create chat: %w", err)
	}

	return model, nil
}

func LocalizationMiddleware(t *i18n.Translator) botapi.Middleware {
	return func(next botapi.Handler) botapi.Handler {
		return func(c *botapi.Context) error {
			ch, err := cctx.Chat(c)
			loc := t.Default()

			if err == nil {
				loc = t.Localizer(ch.Lang)
			}

			c.Context = cctx.WithLocalizer(c.Context, loc)

			return next(c)
		}
	}
}

func getOrCreateUser(
	ctx context.Context,
	repo user.Repository,
	sender *botapi.User,
	c botapi.Chat,
) (model user.User, userUpdate user.UserUpdate, err error) {
	var (
		senderID                                        int64
		senderUsername, senderFirstName, senderLastName string
		senderIsBot                                     bool
	)

	switch {
	case sender != nil:
		senderID = sender.ID
		senderUsername = sender.Username
		senderFirstName = sender.FirstName
		senderLastName = sender.LastName
		senderIsBot = sender.IsBot

	case c.Type == botapi.ChatTypePrivate:
		senderID = c.ID
		senderUsername = c.Username
		senderFirstName = c.FirstName
		senderLastName = c.LastName
	default:
		return model, userUpdate, fmt.Errorf("no user")
	}

	model, err = repo.Get(ctx, senderID)
	if err == nil {
		userUpdate.OldUsername = model.Username

		updated := false

		if model.Username != senderUsername {
			updated = true
			userUpdate.NewUsername = senderUsername
			model.Username = senderUsername
		}

		if model.FirstName != senderFirstName {
			updated = true
			model.FirstName = senderFirstName
		}

		if model.LastName != senderLastName {
			updated = true
			model.LastName = senderLastName
		}

		if updated {
			if err := repo.Update(ctx, model); err != nil {
				return model, userUpdate, err
			}
		}

		return model, userUpdate, nil
	}

	if !errors.Is(err, pgx.ErrNoRows) {
		return model, userUpdate, fmt.Errorf("get user: %w", err)
	}

	model = user.New(
		senderID,
		senderFirstName,
		senderLastName,
		senderUsername,
		user.GenderUnknown,
		senderIsBot,
		time.Now(),
	)

	if err := repo.Create(ctx, model); err != nil {
		return model, userUpdate, fmt.Errorf("create user: %w", err)
	}

	return model, userUpdate, nil
}

func getOrCreateChatMember(
	ctx context.Context,
	repo chatmember.Repository,
	bot *botapi.Bot,
	chatModel chat.Chat,
	userModel user.User,
) (chatmember.ChatMember, error) {
	chatID := chatModel.ID
	userID := userModel.ID

	member, err := repo.Get(ctx, chatID, userID)
	if err == nil {
		return member, nil
	}

	if !errors.Is(err, pgx.ErrNoRows) {
		return chatmember.ChatMember{}, fmt.Errorf("get chat member: %w", err)
	}

	var (
		status permission.Status
		tag    string
	)

	cm, err := bot.GetChatMember(ctx, botapi.ChatIDInt(chatID), userID)
	if err != nil {
		status = permission.StatusMember
		tag = ""
	} else {
		status, tag, err = parseChatMember(cm)
		if err != nil {
			return chatmember.ChatMember{}, err
		}
	}

	member = chatmember.New(
		userModel,
		chatModel,
		tag,
		status,
		time.Now(),
	)

	if err := repo.Create(ctx, member); err != nil {
		return chatmember.ChatMember{}, fmt.Errorf("create chat member: %w", err)
	}

	return member, nil
}

func parseChatMember(cm botapi.ChatMember) (permission.Status, string, error) {
	switch v := cm.(type) {
	case *botapi.ChatMemberOwner:
		return permission.StatusOwner, v.CustomTitle, nil

	case *botapi.ChatMemberAdministrator:
		return permission.StatusMember, v.CustomTitle, nil

	case *botapi.ChatMemberMember:
		return permission.StatusMember, v.Tag, nil

	case *botapi.ChatMemberRestricted:
		return permission.StatusMember, v.Tag, nil

	case *botapi.ChatMemberLeft, *botapi.ChatMemberBanned:
		return permission.StatusMember, "", fmt.Errorf("user is not active chat member")

	default:
		return permission.StatusMember, "", fmt.Errorf("unknown chat member type")
	}
}
