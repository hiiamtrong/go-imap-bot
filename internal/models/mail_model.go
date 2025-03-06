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

// UnixToTime converts Unix timestamp to time.Time
func UnixToTime(timestamp int64) time.Time {
	return time.Unix(timestamp, 0)
}
