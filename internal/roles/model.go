package roles

import "time"

type Fandom struct {
	ID         int64
	Name       string
	Categories []Category
}

type Category struct {
	ID    int64
	Name  string
	Emoji string
	Roles []Role
}

type Role struct {
	ID      int64
	Name    string
	Emoji   string
	Aliases []string
}

type RoleReservation struct {
	ID        int64
	ChatID    int64
	UserID    int64
	CreatedAt time.Time
	Role      Role
}
