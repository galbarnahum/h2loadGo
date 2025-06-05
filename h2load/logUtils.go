package h2load

import (
	"encoding/json"
	"fmt"
	"time"
)

// logResult sends a log line to the logger goroutine
func LogResultAsJSON(start time.Time, status int, latency time.Duration) string {
	entry := map[string]interface{}{
		"timestamp": start.Format("15:04:05.000000000"),
		"status":    status,
		"latency":   fmt.Sprintf("%.3fms", float64(latency.Nanoseconds())/1000000),
	}
	jsonBytes, err := json.Marshal(entry)
	if err != nil {
		return "" // optionally handle or report JSON marshal error
	}
	return string(jsonBytes) + "\n"
}

func LogResultAsText(start time.Time, status int, latency time.Duration) string {
	epochMicros := start.UnixNano() / int64(time.Microsecond)
	return fmt.Sprintf("%d %d %d\n", epochMicros, status, latency.Microseconds())
}
