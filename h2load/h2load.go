package h2load

import (
	"fmt"
	"time"
)

type RpsMode int

const (
	RpsModeBurst RpsMode = iota // fire as fast as allowed up to the RPS limit per second
	RpsModeEven                 // spread requests evenly within the second
)

// the fields that matter are
// requests
// rps
// concurrent streams

type H2loadConf struct {
	Protocol          string
	ServerAddress     string
	Requests          int
	Rate              int
	RatePeriod        int
	Rps               int
	RpsMode           RpsMode
	ConcurrentStreams int
	Clients           int
	URL               string
}

func (h *H2loadConf) Validate() error {
	if h.URL == "" {
		return fmt.Errorf("URL is required")
	}
	if h.Requests < 0 {
		return fmt.Errorf("requests must be greater than 0")
	}
	if h.Rate < 0 {
		return fmt.Errorf("rate must be greater than 0")
	}
	if h.RatePeriod < 0 {
		return fmt.Errorf("rate period must be greater than 0")
	}
	if h.Rps < 0 {
		return fmt.Errorf("rps must be greater than 0")
	}
	if h.ConcurrentStreams < 0 {
		return fmt.Errorf("concurrent streams must be greater than 0")
	}
	if h.Clients < 0 {
		return fmt.Errorf("clients must be greater than 0")
	}
	return nil
}

type LogEntry struct {
	Status    int
	Latency   time.Duration
	Timestamp string
}
