package middleware

import (
	"time"

	"github.com/gotd/botapi"
)

// StartupFilterMiddleware отбрасывает апдейты, датированные раньше startedAt.
// Это позволяет избежать обработки накопившихся офлайн-апдейтов при перезапуске
// и не словить flood-лимиты.
//
// Используй с bot.UseOuter(...) первым в цепочке, чтобы старые апдейты
// не проходили ни через один следующий middleware.
func StartupFilterMiddleware(startedAt time.Time) botapi.Middleware {
	return func(next botapi.Handler) botapi.Handler {
		return func(c *botapi.Context) error {
			if date := updateDate(c); date > 0 {
				if time.Unix(int64(date), 0).Before(startedAt) {
					return nil
				}
			}
			return next(c)
		}
	}
}

// updateDate извлекает Unix-timestamp апдейта (поле Date из сообщения
// или callback-запроса). Возвращает 0, если дата недоступна.
func updateDate(c *botapi.Context) int {
	if msg := c.Message(); msg != nil {
		return msg.Date
	}
	if cq := c.Update.CallbackQuery; cq != nil && cq.Message != nil {
		return cq.Message.Date
	}
	return 0
}
