package h2load

import (
	"fmt"
	"log"
	"net/http"
	"time"
)

type H2loadClient struct {
	Clients     []*H2Client
	ClientsConf H2loadConf
}

/*
func NewH2loadClientWithLogger(conf H2loadConf, rootFolder string, logAsJSON bool) (*H2loadClient, error) {
	if !strings.HasPrefix(rootFolder, "/tmp/") {
		fmt.Println("Root folder must begin with /tmp/")
		return nil, errors.New("root folder must begin with /tmp/")
	}
	_ = os.RemoveAll(rootFolder) //remove old root logs folder
	var logPathName string
	if logAsJSON {
		logPathName = "h2load_log.json"
	} else {
		logPathName = "h2load_raw.log"
	}

	h2loadClient, err := NewH2loadClient(conf)
	if err != nil {
		return nil, err
	}

	for i := 0; i < conf.Clients; i++ {
		// Build path: root/h2load_i/
		dir := filepath.Join(rootFolder, fmt.Sprintf("h2load_%d", i))
		if err := os.MkdirAll(dir, 0755); err != nil {
			return nil, fmt.Errorf("failed to create log dir for client %d: %w", i, err)
		}

		logPath := filepath.Join(dir, logPathName)
		if err := h2loadClient.Clients[i].SetLogFile(logPath); err != nil {
			return nil, fmt.Errorf("failed to set log file for client %d: %w", i, err)
		}
		h2loadClient.Clients[i].LogAsJSON = logAsJSON
	}

	return h2loadClient, nil
}
*/

func NewH2loadClient(conf H2loadConf) (*H2loadClient, error) {
	if err := conf.Validate(); err != nil {
		return nil, err
	}
	clients := make([]*H2Client, 0, conf.Clients)
	for i := 0; i < conf.Clients; i++ {
		client := NewH2Client(conf)
		clients = append(clients, client)
	}
	return &H2loadClient{Clients: clients, ClientsConf: conf}, nil
}

func (h *H2loadClient) Connect() error {
	errs := RunConcurrent(h.Clients, func(c *H2Client) error {
		return c.Connect()
	})
	return JoinIndexedErrors(errs)
}

func (h *H2loadClient) RunRequests(req *http.Request) error {
	return h.RunRequestsFactory(func() *http.Request {
		return req
	})
}

func (h *H2loadClient) Run() error {
	req, _ := http.NewRequest("GET", h.ClientsConf.URL, nil)
	return h.RunRequests(req)
}

func (h *H2loadClient) RunRequestsFactory(factory func() *http.Request) error {
	errs := RunConcurrent(h.Clients, func(c *H2Client) error {
		return c.DoRequestsFactory(factory)
	})
	return JoinIndexedErrors(errs)
}

func (h *H2loadClient) Start() error {
	if err := h.Connect(); err != nil {
		return fmt.Errorf("connect failed: %w", err)
	}
	return h.Run()
}

func (h *H2loadClient) Stop() {
	_ = RunConcurrent(h.Clients, func(c *H2Client) error {
		c.Stop()
		return nil
	})
}

func (h *H2loadClient) Wait() {
	_ = RunConcurrent(h.Clients, func(c *H2Client) error {
		c.Wait()
		return nil
	})
}

func (h *H2loadClient) SetLoggerForClient(clientIndex int, logger *log.Logger) {
	h.Clients[clientIndex].SetLogger(logger)
}

func (h *H2loadClient) SetLogLineFuncForClient(clientIndex int, logLineFunc func(start time.Time, status int, latency time.Duration) string) {
	h.Clients[clientIndex].SetLogLineFunc(logLineFunc)
}

func (h *H2loadClient) SetGlobalLogger(logger *log.Logger) {
	for _, c := range h.Clients {
		c.SetLogger(logger)
	}
}

func (h *H2loadClient) SetGlobalLogLineFunc(logLineFunc func(start time.Time, status int, latency time.Duration) string) {
	for _, c := range h.Clients {
		c.SetLogLineFunc(logLineFunc)
	}
}

func (h *H2loadClient) Close() {
	_ = RunConcurrent(h.Clients, func(c *H2Client) error {
		c.Close()
		return nil
	})
}

func (h *H2loadClient) GetSentRequests() int64 {
	total := int64(0)
	for _, c := range h.Clients {
		total += c.GetSentRequests()
	}
	return total
}

// GetTotalStats returns aggregated total statistics from all clients
func (h *H2loadClient) GetTotalStats() RequestStats {
	var totalStats RequestStats

	for _, client := range h.Clients {
		stats := client.GetStats()
		totalStats.TotalRequests += stats.TotalRequests
		totalStats.SuccessRequests += stats.SuccessRequests
		totalStats.FailedRequests += stats.FailedRequests
		totalStats.TotalLatency += stats.TotalLatency

		// For min latency, take the minimum across all clients (ignore zero values)
		if totalStats.MinLatency == 0 || (stats.MinLatency > 0 && stats.MinLatency < totalStats.MinLatency) {
			totalStats.MinLatency = stats.MinLatency
		}
		// For max latency, take the maximum across all clients
		if stats.MaxLatency > totalStats.MaxLatency {
			totalStats.MaxLatency = stats.MaxLatency
		}
		// For duration, take the maximum (longest running client)
		if stats.Duration > totalStats.Duration {
			totalStats.Duration = stats.Duration
		}
	}

	return totalStats
}

// GetAvgClientStats returns average statistics per client as RequestStats
func (h *H2loadClient) GetAvgClientStats() RequestStats {
	totalStats := h.GetTotalStats()
	clientCount := len(h.Clients)

	// Convert totals to averages per client
	return RequestStats{
		TotalRequests:   int64(float64(totalStats.TotalRequests) / float64(clientCount)),
		SuccessRequests: int64(float64(totalStats.SuccessRequests) / float64(clientCount)),
		FailedRequests:  int64(float64(totalStats.FailedRequests) / float64(clientCount)),
		MinLatency:      totalStats.MinLatency, // Keep min/max as-is (not averages)
		MaxLatency:      totalStats.MaxLatency,
		TotalLatency:    time.Duration(int64(totalStats.TotalLatency) / int64(clientCount)),
		Duration:        totalStats.Duration, // Duration is per test, not per client
	}
}

// GetStatsSummary returns combined statistics summary (both totals and averages)
func (h *H2loadClient) GetStatsSummary() string {
	return h.GetTotalStats().String() + "\n\n" + h.GetAvgClientStats().String()
}

// GetClientStats returns statistics for a specific client
func (h *H2loadClient) GetClientStats(clientIndex int) string {
	if clientIndex < 0 || clientIndex >= len(h.Clients) {
		return fmt.Sprintf("Invalid client index: %d", clientIndex)
	}
	return h.Clients[clientIndex].GetStatsSummary()
}

func (h *H2loadClient) GetAllClientsStatsSummary() string {
	stats := ""
	for i, c := range h.Clients {
		stats += fmt.Sprintf("~~~~~ Client %d ~~~~~ \n\n %s\n\n\n", i, c.GetStatsSummary())
	}
	return stats
}
