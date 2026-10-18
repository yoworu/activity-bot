package middleware

import (
	"activity-bot/internal/cctx"
	"activity-bot/internal/user"
	"fmt"

	"github.com/gotd/botapi"
)

func UserMiddleware(repo user.Repository) botapi.Middleware {
	return func(next botapi.Handler) botapi.Handler {
		return func(c *botapi.Context) error {
			msg := c.Message()
			if msg == nil && c.Update.CallbackQuery != nil {
				msg = c.Update.CallbackQuery.Message
			}

			if msg == nil {
				return next(c)
			}

			model, update, err := getOrCreateUser(
				c,
				repo,
				c.Sender(),
				msg.Chat,
			)
			if err != nil {
				return fmt.Errorf("user middleware: %w", err)
			}

			c.Context = cctx.WithUser(c.Context, model)
			c.Context = cctx.WithUserUpdate(c.Context, update)

			return next(c)
		}
	}
}
