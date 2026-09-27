package application

import (
	"activity-bot/internal/roles"
)

type State string

const (
	AppStateIdle           State = ""
	AppStateAwaitRole      State = "await_role"
	AppStateAwaitBirthDate State = "await_birth_date"
	AppStateConfirmRules   State = "confirm_role"
	AppStatePending        State = "pending"
)

type AppStateData struct {
	CategoryID int64
	Role       roles.Role
	BirthDate  string
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
