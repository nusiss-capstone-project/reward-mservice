package util

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

func TestFormatDateTimeUsesLocalTimezone(t *testing.T) {
	oldLocal := time.Local
	time.Local = time.FixedZone("CST", 8*3600)
	t.Cleanup(func() {
		time.Local = oldLocal
	})

	utcTime := time.Date(2026, 7, 3, 10, 30, 45, 0, time.UTC)
	assert.Equal(t, "2026-07-03 18:30:45", FormatDateTime(utcTime))
}

func TestFormatDateTimeZero(t *testing.T) {
	assert.Equal(t, "", FormatDateTime(time.Time{}))
}
