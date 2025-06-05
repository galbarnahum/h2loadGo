package h2load

import (
	"fmt"
	"time"
)

type RequestStats struct {
	TotalRequests   int64
	SuccessRequests int64
	FailedRequests  int64
	MinLatency      time.Duration
	MaxLatency      time.Duration
	TotalLatency    time.Duration
	Duration        time.Duration
}

// String formats the RequestStats as a readable string
func (r RequestStats) String() string {
	var avgLatency time.Duration
	if r.TotalRequests > 0 {
		avgLatency = r.TotalLatency / time.Duration(r.TotalRequests)
	}

	var rps float64
	if r.Duration > 0 {
		rps = float64(r.TotalRequests) / r.Duration.Seconds()
	}

	return fmt.Sprintf(`Statistics:
Total Requests: %d
Successful Requests: %d
Failed Requests: %d
Requests/sec: %.2f
Min Latency: %v
Max Latency: %v
Average Latency: %v
Total Duration: %v`,
		r.TotalRequests,
		r.SuccessRequests,
		r.FailedRequests,
		rps,
		r.MinLatency,
		r.MaxLatency,
		avgLatency,
		r.Duration)
}
