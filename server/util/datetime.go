package util

import "time"

const DateTimeLayout = "2006-01-02 15:04:05"

// FormatDateTime formats time in local timezone to match MySQL DSN loc=Local.
func FormatDateTime(t time.Time) string {
	if t.IsZero() {
		return ""
	}
	return t.In(time.Local).Format(DateTimeLayout)
}
