package h2load

import (
	"flag"
	"fmt"
	"log"
	"os"
	"strings"
	"time"
)

type CLIConfig struct {
	H2loadConf // Embedded struct for load testing configuration

	// CLI-specific settings
	ShowStats       bool
	ShowClientStats bool
	LogJSON         bool
	LogFile         string
	Duration        time.Duration

	// Help
	ShowHelp bool
}

func ParseFlags() *CLIConfig {
	config := &CLIConfig{}

	// Define flags for H2loadConf fields
	flag.StringVar(&config.URL, "url", "", "Target URL (required)")
	flag.StringVar(&config.URL, "u", "", "Target URL (shorthand)")

	flag.IntVar(&config.Requests, "requests", 1, "Number of requests per client")
	flag.IntVar(&config.Requests, "n", 1, "Number of requests per client (shorthand)")

	flag.IntVar(&config.Clients, "clients", 1, "Number of concurrent clients")
	flag.IntVar(&config.Clients, "c", 1, "Number of concurrent clients (shorthand)")

	flag.IntVar(&config.ConcurrentStreams, "streams", 1, "Number of concurrent streams per client")
	flag.IntVar(&config.ConcurrentStreams, "s", 1, "Number of concurrent streams per client (shorthand)")

	flag.IntVar(&config.Rps, "rps", 0, "Requests per second (0 = unlimited)")
	flag.IntVar(&config.Rps, "r", 0, "Requests per second (shorthand)")

	var rpsMode string
	flag.StringVar(&rpsMode, "rps-mode", "burst", "RPS mode: 'burst' or 'even'")
	flag.StringVar(&config.ServerAddress, "server", "", "Server address override (host:port)")
	flag.StringVar(&config.Protocol, "protocol", "", "Protocol override")

	// CLI-specific flags
	flag.BoolVar(&config.ShowStats, "stats", true, "Show aggregated statistics")
	flag.BoolVar(&config.ShowClientStats, "client-stats", false, "Show individual client statistics")
	flag.BoolVar(&config.LogJSON, "json", false, "Output logs in JSON format")
	flag.StringVar(&config.LogFile, "log-file", "", "Log file path (logs to stdout if not specified)")
	flag.DurationVar(&config.Duration, "duration", 0, "Test duration (overrides -n requests)")

	flag.BoolVar(&config.ShowHelp, "help", false, "Show help message")
	flag.BoolVar(&config.ShowHelp, "h", false, "Show help message (shorthand)")

	// Custom usage function
	flag.Usage = func() {
		fmt.Fprintf(os.Stderr, "H2loadClient - HTTP/2 Load Testing Tool\n\n")
		fmt.Fprintf(os.Stderr, "Usage: %s [options]\n\n", os.Args[0])
		fmt.Fprintf(os.Stderr, "Required:\n")
		fmt.Fprintf(os.Stderr, "  -url, -u <url>          Target URL to test\n\n")
		fmt.Fprintf(os.Stderr, "Load Options:\n")
		fmt.Fprintf(os.Stderr, "  -requests, -n <int>     Number of requests per client (default: 1)\n")
		fmt.Fprintf(os.Stderr, "  -clients, -c <int>      Number of concurrent clients (default: 1)\n")
		fmt.Fprintf(os.Stderr, "  -streams, -s <int>      Number of concurrent streams per client (default: 1)\n")
		fmt.Fprintf(os.Stderr, "  -rps, -r <int>          Requests per second limit (0 = unlimited, default: 0)\n")
		fmt.Fprintf(os.Stderr, "  -rps-mode <mode>        RPS mode: 'burst' or 'even' (default: burst)\n")
		fmt.Fprintf(os.Stderr, "  -duration <duration>    Test duration (e.g. 30s, 1m) - overrides -n\n\n")
		fmt.Fprintf(os.Stderr, "Connection Options:\n")
		fmt.Fprintf(os.Stderr, "  -server <host:port>     Override server address\n")
		fmt.Fprintf(os.Stderr, "  -protocol <protocol>    Protocol override\n\n")
		fmt.Fprintf(os.Stderr, "Output Options:\n")
		fmt.Fprintf(os.Stderr, "  -stats                  Show aggregated statistics (default: true)\n")
		fmt.Fprintf(os.Stderr, "  -client-stats           Show individual client statistics (default: false)\n")
		fmt.Fprintf(os.Stderr, "  -json                   Output logs in JSON format (default: false)\n")
		fmt.Fprintf(os.Stderr, "  -log-file <path>        Log file path (logs to stdout if not specified)\n\n")
		fmt.Fprintf(os.Stderr, "Help:\n")
		fmt.Fprintf(os.Stderr, "  -help, -h               Show this help message\n\n")
		fmt.Fprintf(os.Stderr, "Examples:\n")
		fmt.Fprintf(os.Stderr, "  %s -url https://example.com -n 100 -c 10\n", os.Args[0])
		fmt.Fprintf(os.Stderr, "  %s -u https://api.example.com -n 1000 -c 50 -s 20 -rps 100\n", os.Args[0])
		fmt.Fprintf(os.Stderr, "  %s -url https://example.com -duration 30s -c 10 -rps-mode even\n", os.Args[0])
		fmt.Fprintf(os.Stderr, "  %s -url https://example.com -n 100 -c 10 -log-file results.log -json\n", os.Args[0])
	}

	flag.Parse()

	// Convert RPS mode string to enum
	if strings.ToLower(rpsMode) == "even" {
		config.RpsMode = RpsModeEven
	} else {
		config.RpsMode = RpsModeBurst
	}

	return config
}

func (c *CLIConfig) Validate() error {
	return c.H2loadConf.Validate()
}

func (c *CLIConfig) GetRpsModeString() string {
	if c.RpsMode == RpsModeEven {
		return "even"
	}
	return "burst"
}

func CLIMain() {
	config := ParseFlags()
	if config.ShowHelp {
		flag.Usage()
		os.Exit(0)
	}

	if err := config.Validate(); err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n\n", err)
		os.Exit(1)
	}

	// Handle duration override
	if config.Duration > 0 {
		// When duration is specified, we'll run indefinitely and stop after duration
		config.Requests = 0 // 0 means run indefinitely
	}

	// Create client
	client, err := NewH2loadClient(config.H2loadConf)
	if err != nil {
		log.Fatalf("Failed to create h2load client: %v", err)
	}
	defer client.Close()

	// Set up logging if needed
	var logger *log.Logger
	var logFile *os.File

	if config.LogFile != "" || config.LogJSON {
		if config.LogFile != "" {
			// Create or open log file
			logFile, err = os.Create(config.LogFile)
			if err != nil {
				log.Fatalf("Failed to create log file %s: %v", config.LogFile, err)
			}
			defer logFile.Close()
			logger = log.New(logFile, "", 0) // No prefix/timestamp for clean logs
			fmt.Printf("Logging to file: %s\n", config.LogFile)
		} else {
			// Log to stdout
			logger = log.Default()
		}
		logger.SetFlags(0)
		client.SetGlobalLogger(logger)

		if config.LogJSON {
			client.SetGlobalLogLineFunc(LogResultAsJSON)
			fmt.Printf("Starting H2load test with JSON logging...\n")
		} else {
			client.SetGlobalLogLineFunc(LogResultAsText)
			fmt.Printf("Starting H2load test with text logging...\n")
		}
	} else {
		fmt.Printf("Starting H2load test...\n")
	}

	// Print configuration
	fmt.Printf("Configuration:\n")
	fmt.Printf("  URL: %s\n", config.URL)
	fmt.Printf("  Clients: %d\n", config.Clients)
	fmt.Printf("  Requests per client: %d\n", config.Requests)
	fmt.Printf("  Concurrent streams per client: %d\n", config.ConcurrentStreams)
	fmt.Printf("  RPS: %d (%s mode)\n", config.Rps, config.GetRpsModeString())
	if config.Duration > 0 {
		fmt.Printf("  Duration: %v\n", config.Duration)
	}
	if config.LogFile != "" {
		fmt.Printf("  Log file: %s\n", config.LogFile)
	}
	fmt.Printf("\n")

	// Connect and start the test
	if err := client.Connect(); err != nil {
		log.Fatalf("Failed to connect: %v", err)
	}

	// Start the test
	startTime := time.Now()

	if config.Duration > 0 {
		// Run for specified duration
		go func() {
			if err := client.Run(); err != nil {
				log.Printf("Test error: %v", err)
			}
		}()

		// Wait for duration
		time.Sleep(config.Duration)
		client.Stop()
	} else {
		// Run until requests are completed
		if err := client.Start(); err != nil {
			log.Fatalf("Test failed: %v", err)
		}
	}

	// Wait for all operations to complete
	client.Wait()

	testDuration := time.Since(startTime)
	fmt.Printf("\nTest completed in %v\n\n", testDuration)

	if config.LogFile != "" {
		fmt.Printf("Request logs written to: %s\n\n", config.LogFile)
	}

	// Show statistics
	if config.ShowStats {
		fmt.Println(client.GetStatsSummary())
		fmt.Println()
	}

	if config.ShowClientStats {
		fmt.Println("Individual Client Statistics:")
		fmt.Println("=" + strings.Repeat("=", 40))
		for i := 0; i < len(client.Clients); i++ {
			fmt.Printf("\nClient %d:\n", i)
			fmt.Println(client.GetClientStats(i))
		}
	}
}
