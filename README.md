# H2loadClient - HTTP/2 Load Testing Tool

An HTTP/2 load testing client equivalent to h2load by nghttp2, but written in go.
Includes comprehensive statistics and configurable concurrency options.

## Features

- **HTTP/2 Support**: Native HTTP/2 client with configurable streams
- **Concurrent Testing**: Multiple clients with configurable concurrent streams
- **Rate Limiting**: RPS control with burst and even distribution modes
- **Real-time Statistics**: Detailed performance metrics and statistics
- **Flexible Logging**: JSON and text logging options
- **CLI Interface**: Easy-to-use command line interface
- **Library Support**: Use as a Go library in your applications


## Usage

### CLI Usage

#### Basic Examples

```bash
# Simple test with 100 requests using 10 clients
./h2load-cli -url https://example.com -n 100 -c 10

# Test with rate limiting (100 RPS)
./h2load-cli -url https://api.example.com -n 1000 -c 50 -rps 100

# Duration-based test (run for 30 seconds)
./h2load-cli -url https://example.com -duration 30s -c 10
```

#### Command Line Options

**Required:**
- `-url, -u <url>` - Target URL to test

**Load Options:**
- `-requests, -n <int>` - Number of requests per client (default: 1)
- `-clients, -c <int>` - Number of concurrent clients (default: 1)
- `-streams, -s <int>` - Number of concurrent streams per client (default: 1)
- `-rps, -r <int>` - Requests per second limit (0 = unlimited, default: 0)
- `-rps-mode <mode>` - RPS mode: 'burst' or 'even' (default: burst)
- `-duration <duration>` - Test duration (e.g. 30s, 1m) - overrides -n

**Connection Options:**
- `-server <host:port>` - Override server address
- `-protocol <protocol>` - Protocol override

**Output Options:**
- `-stats` - Show aggregated statistics (default: true)
- `-client-stats` - Show individual client statistics (default: false)
- `-json` - Output logs in JSON format (default: false)

**Help:**
- `-help, -h` - Show help message

### Library Usage

#### Using the CLI Function
```go
package main

import "h2loadClient/h2load"

func main() {
    // This will use command line arguments
    h2load.CLIMain()
}
```

#### Programmatic Usage
```go
package main

import (
    "fmt"
    "h2loadClient/h2load"
    "log"
)

func main() {
    conf := h2load.H2loadConf{
        URL:               "https://example.com",
        Clients:           10,
        Requests:          100,
        ConcurrentStreams: 5,
        Rps:               50,
        RpsMode:           h2load.RpsModeEven,
    }

    client, err := h2load.NewH2loadClient(conf)
    if err != nil {
        log.Fatal(err)
    }
    defer client.Close()

    if err := client.Start(); err != nil {
        log.Fatal(err)
    }

    client.Wait()
    fmt.Println(client.GetStatsSummary())
}
```

#### Custom CLI Configuration
```go
package main

import (
    "h2loadClient/h2load"
    "log"
)

func main() {
    // Parse flags and get CLI config
    config := h2load.ParseFlags()
    
    // Access embedded H2loadConf
    conf := config.H2loadConf
    
    // Modify configuration programmatically
    conf.Requests = 500
    
    // Create and run client
    client, err := h2load.NewH2loadClient(conf)
    if err != nil {
        log.Fatal(err)
    }
    defer client.Close()
    
    client.Start()
    client.Wait()
    
    if config.ShowStats {
        fmt.Println(client.GetStatsSummary())
    }
}
```

## Advanced Examples

### Rate-Limited Test
```bash
./h2load-cli -url https://api.example.com -n 5000 -c 20 -rps 200 -rps-mode even
```

### High Concurrency Test
```bash
./h2load-cli -url https://example.com -n 10000 -c 100 -s 50
```

### Duration-Based Test with JSON Logging
```bash
./h2load-cli -url https://api.example.com -duration 1m -c 25 -rps 500 -json
```

### Custom Server Address
```bash
./h2load-cli -url https://example.com -server 192.168.1.100:8443 -n 1000 -c 10
```

## Output Examples

### Configuration Display
```
Configuration:
  URL: https://example.com
  Clients: 10
  Requests per client: 100
  Concurrent streams per client: 1
  RPS: 50 (even mode)
```

### Aggregated Statistics
```
Aggregated Statistics Summary:
=== TOTALS ===
Total Clients: 10
Total Requests: 1000
Successful Requests: 995
Failed Requests: 5
Total Requests/sec: 167.23
Min Latency: 12.3ms
Max Latency: 245.7ms
Average Latency: 45.2ms
Total Duration: 5.978s

=== AVERAGES PER CLIENT ===
Avg Requests per Client: 100.0
Avg Successful per Client: 99.5
Avg Failed per Client: 0.5
Avg RPS per Client: 16.72
```

### Individual Client Statistics (with -client-stats)
```
Individual Client Statistics:
========================================

Client 0:
Total Requests: 100
Successful Requests: 98
Failed Requests: 2
Requests/sec: 16.85
Min Latency: 15.2ms
Max Latency: 198.3ms
Average Latency: 43.7ms
Total Duration: 5.934s
```

## Architecture

The CLI uses an embedded `H2loadConf` struct to avoid duplication:

```go
type CLIConfig struct {
    H2loadConf                    // Embedded configuration
    
    // CLI-specific settings
    ShowStats       bool
    ShowClientStats bool
    LogJSON         bool
    Duration        time.Duration
    ShowHelp        bool
}
```

This design provides:
- **No Duplication**: All H2loadConf fields are directly accessible
- **Clean Separation**: CLI-specific options are separate
- **Type Safety**: Direct use of enums and validation
- **Extensibility**: Easy to add new CLI-only features

## RPS Modes

- **Burst Mode** (`-rps-mode burst`): Sends all allowed requests at the beginning of each second
- **Even Mode** (`-rps-mode even`): Distributes requests evenly throughout each second

## Performance Tips

1. **Optimal Client Count**: Start with 10-50 clients and adjust based on your target server's capacity
2. **Stream Concurrency**: Use 1-20 streams per client for most scenarios
3. **Rate Limiting**: Use even mode for more consistent load distribution
4. **Duration vs Requests**: Use duration-based testing for sustained load testing
5. **Statistics**: Enable client stats only when debugging individual client performance

## Requirements

- Go 1.19 or later
- Network connectivity to target servers
- For HTTPS: Target server must support HTTP/2 