package h2load

import (
	"context"
	"crypto/tls"
	"fmt"
	"io"
	"log"
	"net"
	"net/http"
	urlpkg "net/url"
	"sync"
	"sync/atomic"
	"time"

	"golang.org/x/net/http2"
)

// Assuming RpsMode, H2loadLogEntry, and H2loadConf are defined in h2load.go
// and that H2loadConf in h2load.go includes the logger, logChan, and logWg fields.

type H2Client struct {
	Conf         H2loadConf
	LogAsJSON    bool
	LogLineFunc  func(start time.Time, status int, latency time.Duration) string
	client       *http.Client
	ctx          context.Context
	cancel       context.CancelFunc
	sentRequests int64

	logger    *log.Logger    // Logger instance for this client
	logChan   chan string    // Channel for asynchronous logging
	loggingWg sync.WaitGroup // WaitGroup for logging operations
	reqWg     sync.WaitGroup // WaitGroup for requests
	stats     RequestStats   // Statistics for this client
	statsChan chan LogEntry  // Channel for asynchronous stats collection
	statsWg   sync.WaitGroup // WaitGroup for stats collection
}

func NewH2Client(conf H2loadConf) *H2Client {
	ctx, cancel := context.WithCancel(context.Background())
	// Validate URL early
	if _, err := urlpkg.Parse(conf.URL); err != nil {
		panic(fmt.Errorf("invalid URL in conf: %w", err))
	}

	h := &H2Client{
		Conf:        conf,
		ctx:         ctx,
		cancel:      cancel,
		logger:      nil,
		logChan:     make(chan string, 10000),
		loggingWg:   sync.WaitGroup{},
		reqWg:       sync.WaitGroup{},
		LogLineFunc: LogResultAsJSON,
		stats:       RequestStats{},
		statsChan:   make(chan LogEntry, 10000),
		statsWg:     sync.WaitGroup{},
	}

	// Start the stats collector goroutine
	h.statsWg.Add(1)
	go func() {
		defer h.statsWg.Done()
		h.statsCollector()
	}()

	return h
}

func (h *H2Client) statsCollector() {
	for entry := range h.statsChan {
		h.stats.TotalRequests++
		if entry.Status >= 200 && entry.Status < 400 {
			h.stats.SuccessRequests++
		} else {
			h.stats.FailedRequests++
		}

		if h.stats.TotalRequests == 1 {
			h.stats.MinLatency = entry.Latency
			h.stats.MaxLatency = entry.Latency
		} else {
			if entry.Latency < h.stats.MinLatency {
				h.stats.MinLatency = entry.Latency
			}
			if entry.Latency > h.stats.MaxLatency {
				h.stats.MaxLatency = entry.Latency
			}
		}
		h.stats.TotalLatency += entry.Latency
	}
}

// logStats sends stats to the stats collector goroutine
func (h *H2Client) logStats(status int, latency time.Duration) {
	select {
	case h.statsChan <- LogEntry{Status: status, Latency: latency, Timestamp: ""}:
		// sent successfully
	default:
		// drop silently if the channel is full
	}
}

func (h *H2Client) closeChannels() {
	close(h.statsChan)
	close(h.logChan)
}

func (h *H2Client) Stop() {
	h.cancel()
	h.Wait()
}

func (h *H2Client) Wait() {
	h.reqWg.Wait()
	h.loggingWg.Wait()
	h.statsWg.Wait()
}

// Connect sets up the HTTP/2 client
func (h *H2Client) Connect() error {
	dialAddr := h.Conf.ServerAddress
	if dialAddr == "" {
		parsed, err := urlpkg.Parse(h.Conf.URL)
		if err != nil {
			return fmt.Errorf("invalid URL: %w", err)
		}
		dialAddr = parsed.Host
	}

	parsed, err := urlpkg.Parse(h.Conf.URL)
	if err != nil {
		return fmt.Errorf("invalid URL: %w", err)
	}
	useTLS := parsed.Scheme == "https"

	if useTLS {
		tlsConfig := &tls.Config{
			InsecureSkipVerify: true,
			ServerName:         getHostname(h.Conf.URL),
			NextProtos:         []string{"h2"},
		}
		transport := &http2.Transport{
			TLSClientConfig: tlsConfig,
			DialTLS: func(network, _ string, cfg *tls.Config) (net.Conn, error) {
				return tls.Dial(network, dialAddr, cfg)
			},
		}
		h.client = &http.Client{Transport: transport}
	} else {
		transport := &http2.Transport{
			AllowHTTP: true,
			DialTLS: func(network, _ string, _ *tls.Config) (net.Conn, error) {
				return net.Dial(network, dialAddr)
			},
		}
		h.client = &http.Client{Transport: transport}
	}
	return nil
}

// SetLogger sets the logger to be used and starts the logger goroutine
func (h *H2Client) SetLogger(logger *log.Logger) error {
	if logger == nil {
		return nil
	}

	// If we already have a logger, unset it first
	if h.logChan != nil {
		close(h.logChan)
	}

	h.logger = logger
	h.logChan = make(chan string, 10000)

	// Start the new logger goroutine
	h.loggingWg.Add(1)
	go func() {
		defer h.loggingWg.Done()
		for line := range h.logChan {
			if h.logger != nil {
				h.logger.Print(line)
			}
		}
	}()
	return nil
}

func (h *H2Client) SetLogLineFunc(logLineFunc func(start time.Time, status int, latency time.Duration) string) {
	h.LogLineFunc = logLineFunc
}

func (h *H2Client) logResult(start time.Time, status int, latency time.Duration) {
	h.logStats(status, latency)
	if h.logChan == nil || h.logger == nil {
		return // No logger channel is set up
	}
	logLine := h.LogLineFunc(start, status, latency)
	// Send the formatted line to the channel
	select {
	case h.logChan <- logLine:
		// sent successfully
	default:
		// drop silently if the channel is full
	}
}

func (h *H2Client) DoRequest(req *http.Request) (*http.Response, error) {
	start := time.Now()
	resp, err := h.client.Do(req)
	latency := time.Since(start)

	if err != nil {
		h.logResult(start, 0, latency)
		return nil, fmt.Errorf("request failed: %w", err)
	}

	io.Copy(io.Discard, resp.Body)
	resp.Body.Close()

	h.logResult(start, resp.StatusCode, latency)
	return resp, nil
}

// DoRequests sends as many requests as possible, never exceeding maxStreams in flight
func (h *H2Client) DoRequests(req *http.Request) {
	//req.Host = getHostname(h.Conf.URL) // override Host header
	h.DoRequestsFactory(func() *http.Request {
		// clone the request to make it safe for reuse
		newReq := req.Clone(req.Context())
		return newReq
	})
}

// DoRequests sends as many requests as possible, never exceeding maxStreams in flight
func (h *H2Client) DoRequestsAsync(req *http.Request) {
	h.reqWg.Add(1)
	go func() {
		defer h.reqWg.Done()
		h.DoRequests(req)
	}()
}

func (h *H2Client) DoRequestsFactoryAsync(factory func() *http.Request) error {
	h.reqWg.Add(1)
	go func() {
		defer h.reqWg.Done()
		h.DoRequestsFactory(factory)
	}()
	return nil
}
func (h *H2Client) DoRequestsFactory(factory func() *http.Request) error {
	defer h.closeChannels()
	streams := make(chan struct{}, h.Conf.ConcurrentStreams)
	defer close(streams)
	var streamsWg sync.WaitGroup
	var firstErr atomic.Value

	// RPS limiter setup
	var rpsTokens chan struct{}
	var rpsResetTicker *time.Ticker

	if h.Conf.Rps > 0 {
		rpsTokens = make(chan struct{}, h.Conf.Rps)
		defer close(rpsTokens)
		rpsResetTicker = time.NewTicker(time.Second)
		defer rpsResetTicker.Stop()

		// For even mode, we'll need a separate ticker
		var evenTicker *time.Ticker
		if h.Conf.RpsMode == RpsModeEven {
			interval := time.Second / time.Duration(h.Conf.Rps)
			evenTicker = time.NewTicker(interval)
			defer evenTicker.Stop()

			// Start a goroutine to continuously fill tokens at even intervals
			go func() {
				for range evenTicker.C {
					select {
					case <-h.ctx.Done():
						return
					case rpsTokens <- struct{}{}:
					default:
						// If channel is full, skip this token
					}
				}
			}()
		}

		// For burst mode or to reset even mode's counter
		go func() {
			for range rpsResetTicker.C {
				if h.Conf.RpsMode == RpsModeBurst {
					// Fill the channel all at once for burst mode
					for i := 0; i < h.Conf.Rps; i++ {
						select {
						case <-h.ctx.Done():
							return
						case rpsTokens <- struct{}{}:
						default:
							// If channel is full, skip this token
						}
					}
				}
			}
		}()
	}

	startTime := time.Now()
loop:
	for {
		select {
		case <-h.ctx.Done():
			break loop
		default:
			// Check if we've sent the requested number of requests
			// If Requests is 0, continue indefinitely
			if h.Conf.Requests > 0 && atomic.LoadInt64(&h.sentRequests) >= int64(h.Conf.Requests) {
				break loop
			}

			// Wait for RPS token if rate limiting is enabled
			if h.Conf.Rps > 0 {
				select {
				case <-h.ctx.Done():
					break loop
				case <-rpsTokens:
					// Got RPS token, continue
				}
			}

			select {
			case <-h.ctx.Done():
				break loop
			case streams <- struct{}{}:
				atomic.AddInt64(&h.sentRequests, 1)
				streamsWg.Add(1)
				go func() {
					defer func() {
						<-streams
						streamsWg.Done()
					}()
					req := factory()
					_, err := h.DoRequest(req)
					if err != nil && firstErr.Load() == nil {
						firstErr.Store(err)
					}
				}()
			default:
				time.Sleep(time.Microsecond)
			}
		}
	}
	streamsWg.Wait()
	h.stats.Duration = time.Since(startTime)
	if errVal := firstErr.Load(); errVal != nil {
		return errVal.(error)
	}
	return nil
}

// Close stops the client and signals the shared logger goroutine to finish.
func (h *H2Client) Close() {
	h.Stop()
	h.client.CloseIdleConnections()
}

func (h *H2Client) GetSentRequests() int64 {
	return atomic.LoadInt64(&h.sentRequests)
}

// GetStats returns a copy of the current statistics
func (h *H2Client) GetStats() RequestStats {
	return h.stats
}

// GetStatsSummary returns a formatted string with statistics
func (h *H2Client) GetStatsSummary() string {
	return h.GetStats().String()
}
