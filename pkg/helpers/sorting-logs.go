package helpers

import (
	"sort"
	"strconv"
	"time"
)

type logEntry struct {
	timestamp time.Time
	message   string
}

func sortingLogs(lokiResponse *LokiResponse) []string {
	var entries []logEntry
	for _, result := range lokiResponse.Data.Result {
		for _, value := range result.Values {
			// Parse timestamp (value[0] is a string in nanoseconds since epoch)
			ns, err := strconv.ParseInt(value[0], 10, 64)
			if err != nil {
				continue
			}
			timestamp := time.Unix(0, ns)
			entries = append(entries, logEntry{
				timestamp: timestamp,
				message:   value[1],
			})
		}
	}
	// Sort entries by timestamp
	sort.Slice(entries, func(i, j int) bool {
		return entries[i].timestamp.Before(entries[j].timestamp)
	})

	// Extract sorted messages
	logs := make([]string, len(entries))
	for i, entry := range entries {
		logs[i] = entry.message
	}
	return logs
}
