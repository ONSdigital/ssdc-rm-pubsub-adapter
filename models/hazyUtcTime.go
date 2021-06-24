package models

import (
	"strings"
	"time"
)

type HazyUtcTime struct {
	// For use in JSON models where the time may or may not include the timezone
	time.Time
}

func (t *HazyUtcTime) UnmarshalJSON(buf []byte) error {
	timeString := strings.Trim(string(buf), `"`)
	parsedTime, err := time.Parse("2006-01-02T15:04:05Z07:00", timeString)
	if err != nil {
		parsedTime, err = time.Parse("2006-01-02T15:04:05", timeString)
		if err != nil {
			return err
		}
	}
	t.Time = parsedTime
	return nil
}
