package application2

import (
	"activity-bot/internal/roles"
)

type State string

const (
	StateIdle             State = ""
	StateAwaitRole        State = "await_role"
	StateAwaitAtmosphere  State = "await_atmosphere"
	StateAwaitWasInFloods State = "await_was_in_floods"
	StateAwaitLeaveReason State = "await_leave_reason"
	StatePending          State = "pending"
)

type StateData struct {
	CategoryID  int64
	Role        roles.Role
	Atmosphere  string
	WasInFloods bool
	LeaveReason string
}

type RejectState string

const (
	RejectStateIdle        RejectState = ""
	RejectStateAwaitReason RejectState = "await_reject_reason"
)

type RejectStateData struct {
	UserID int64 `json:"user_id"`
	ChatID int64 `json:"chat_id"`
}
