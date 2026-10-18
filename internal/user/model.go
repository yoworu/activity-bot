package user

import (
	"activity-bot/internal/emoji"
	"fmt"
	"strings"
	"time"
)

type Gender string

const (
	GenderMale    Gender = "male"
	GenderFemale  Gender = "female"
	GenderUnknown Gender = "unknown"
)

type User struct {
	ID        int64
	FirstName string
	LastName  string
	Username  string
	Gender    Gender
	Emojis    string
	IsBot     bool
	CreatedAt time.Time
}

type UserUpdate struct {
	OldUsername string
	NewUsername string
}

func New(id int64, firstName, lastName, username string, gender Gender, isBot bool, now time.Time) User {
	return User{
		ID:        id,
		FirstName: firstName,
		LastName:  lastName,
		Username:  username,
		Gender:    gender,
		IsBot:     isBot,
		Emojis:    emoji.Random(),
		CreatedAt: now,
	}
}

func (u User) FullName() string {
	return strings.TrimSpace(fmt.Sprintf("%s %s", u.FirstName, u.LastName))
}

func (u User) IsMale() bool {
	return u.Gender == GenderMale
}

func (u User) IsFemale() bool {
	return u.Gender == GenderFemale
}
