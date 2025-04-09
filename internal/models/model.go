package models

import "time"

// UnixToTime converts Unix timestamp to time.Time
func UnixToTime(timestamp int64) time.Time {
	return time.Unix(timestamp, 0)
}
