package models

import "time"

type Mail struct {
	ID      int64
	UID     int64
	Subject string
	From    string
	To      string
	Date    time.Time
}
