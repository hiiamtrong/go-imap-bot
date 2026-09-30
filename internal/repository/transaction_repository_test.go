package repository

import (
	"testing"
	"time"
)

func TestParseTimestamp(t *testing.T) {
	want := time.Date(2026, 9, 26, 12, 21, 30, 0, time.FixedZone("", 7*3600))
	tests := []struct {
		name    string
		in      string
		want    time.Time
		wantErr bool
	}{
		{"legacy unix seconds", "1743476285", time.Unix(1743476285, 0), false},
		{"go layout with offset", "2026-09-26 12:21:30+07:00", want, false},
		{"rfc3339", "2026-09-26T12:21:30+07:00", want, false},
		{"empty", "", time.Time{}, true},
		{"garbage", "yesterday", time.Time{}, true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := parseTimestamp(tt.in)
			if (err != nil) != tt.wantErr {
				t.Fatalf("err = %v, wantErr %v", err, tt.wantErr)
			}
			if err == nil && !got.Equal(tt.want) {
				t.Errorf("got %v, want %v", got, tt.want)
			}
		})
	}
}
