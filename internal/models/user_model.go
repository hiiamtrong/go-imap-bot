package models

import "time"

type User struct {
	ID            int64
	Name          string
	Email         string
	CreatedAt     time.Time
	IsWhitelisted bool
}
