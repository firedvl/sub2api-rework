package service

import "time"

func isJSONTimeInRange(value *time.Time) bool {
	return value == nil || (value.Year() >= 0 && value.Year() <= 9999)
}
