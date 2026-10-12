package application

import (
	"activity-bot/internal/roles"
	"time"
)

type Application struct {
	UserID    int64      `json:"user_id"`
	ChatID    int64      `json:"chat_id"`
	Role      roles.Role `json:"role"`
	Username  string     `json:"username"`
	BirthDate time.Time  `json:"birth_date"`
}
