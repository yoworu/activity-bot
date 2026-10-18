package application

import (
	"activity-bot/internal/roles"
)

type State string

const (
	AppStateIdle              State = ""
	AppStateAwaitAction       State = "await_action"
	AppStateAwaitRole         State = "await_role"
	AppStateAwaitBirthDate    State = "await_birth_date"
	AppStateAwaitReserveUntil State = "await_reserve_until"
	AppStateConfirmRules      State = "confirm_role"
	AppStatePending           State = "pending"
)

type AppStateData struct {
	Kind         ApplicationKind
	CategoryID   int64
	Role         roles.Role
	BirthDate    string
	ReserveUntil string
}

func (d AppStateData) isReserve() bool {
	return d.Kind == KindReserve
}

type RejectState string
type RejectStateData struct {
	UserID int64 `json:"user_id"`
	ChatID int64 `json:"chat_id"`
}

const (
	RejectStateIdle               RejectState = ""
	RejectStateAwaitRejectMessage RejectState = "await_reject_message"
)
