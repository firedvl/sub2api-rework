package service

import "time"

func ollamaCloudUsageExhaustionResetAt(
	snapshot *OllamaCloudUsageSnapshot,
	now time.Time,
	minFetchedAt time.Time,
) (reset time.Time, ok bool) {
	if snapshot == nil || snapshot.Status != OllamaCloudUsageStatusOK || snapshot.Data == nil {
		return time.Time{}, false
	}
	if snapshot.FetchedAt == nil || snapshot.FetchedAt.IsZero() || snapshot.FetchedAt.Before(minFetchedAt) {
		return time.Time{}, false
	}

	var latest time.Time
	for _, window := range []*OllamaCloudUsageWindow{
		snapshot.Data.FiveHour,
		snapshot.Data.SevenDay,
	} {
		if window == nil || window.UsedPercent < 100 {
			continue
		}
		if window.ResetAt == nil || window.ResetAt.IsZero() || !window.ResetAt.After(now) {
			return time.Time{}, false
		}
		candidate := window.ResetAt.UTC()
		if latest.IsZero() || candidate.After(latest) {
			latest = candidate
		}
	}
	if latest.IsZero() {
		return time.Time{}, false
	}
	return latest, true
}
