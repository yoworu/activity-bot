package application2

import (
	"activity-bot/internal/roles"
)

type Application struct {
	UserID      int64      `json:"user_id"`
	ChatID      int64      `json:"chat_id"`
	Role        roles.Role `json:"role"`
	Username    string     `json:"username"`
	Atmosphere  string     `json:"atmosphere"`
	WasInFloods bool       `json:"was_in_floods"`
	LeaveReason string     `json:"leave_reason,omitempty"`
}
