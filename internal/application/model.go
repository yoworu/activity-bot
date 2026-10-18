package application

import (
	"activity-bot/internal/roles"
	"time"
)

type ApplicationKind string

const (
	KindJoin    ApplicationKind = "join"
	KindReserve ApplicationKind = "reserve"
)

type Application struct {
	UserID       int64           `json:"user_id"`
	ChatID       int64           `json:"chat_id"`
	Kind         ApplicationKind `json:"kind"`
	Role         roles.Role      `json:"role"`
	Username     string          `json:"username"`
	BirthDate    time.Time       `json:"birth_date"`
	ReserveUntil time.Time       `json:"reserve_until,omitempty"`
}

func (a Application) IsJoin() bool {
	return a.Kind == KindJoin || a.Kind == ""
}

func (a Application) IsReserve() bool {
	return a.Kind == KindReserve
}
