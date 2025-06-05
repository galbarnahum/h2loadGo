package main

import (
	"fmt"
	"h2loadGo/h2load"
	"log"
	"net/http"
	"time"
)

func main() {
	//main_h2client()
	//main_h2load()
	h2load.CLIMain()
}

func main_h2load() {
	h2loadConf := h2load.H2loadConf{
		Clients:           4,
		Requests:          500,
		Rps:               100,
		RpsMode:           h2load.RpsModeEven,
		ConcurrentStreams: 100,
		URL:               "https://192.168.205.131:8443",
	}

	clients, err := h2load.NewH2loadClient(h2loadConf)
	if err != nil {
		log.Fatalf("init failed: %v", err)
	}

	l := log.Default()
	l.SetFlags(0)
	clients.SetGlobalLogger(l)
	clients.SetGlobalLogLineFunc(h2load.LogResultAsJSON)

	clients.Connect()
	clients.Run()
	clients.Wait()

	// Get structured data
	totalStats := clients.GetTotalStats()
	avgStats := clients.GetAvgClientStats()

	fmt.Printf("Total requests: %d, RPS: %.2f\n",
		totalStats.TotalRequests,
		float64(totalStats.TotalRequests)/totalStats.Duration.Seconds())
	fmt.Printf("Avg per client: %d requests\n", avgStats.TotalRequests)

	// Or get formatted strings
	fmt.Println(totalStats)
}

func main_h2client() {
	h2loadConf := h2load.H2loadConf{
		Requests:          300,
		Rps:               100,
		RpsMode:           h2load.RpsModeBurst,
		ConcurrentStreams: 100,
		URL:               "https://192.168.205.131:8443",
	}
	client := h2load.NewH2Client(h2loadConf)

	client.SetLogger(log.Default())

	if err := client.Connect(); err != nil {
		log.Fatalf("Connect failed: %v", err)
	}

	req, _ := http.NewRequest("GET", client.Conf.URL, nil)
	client.DoRequestsAsync(req)

	time.Sleep(3 * time.Second)
	fmt.Println("stopping client")
	client.Stop()
	fmt.Println("waiting for client to finish...")
	client.Wait()
	fmt.Println("client stopped")

	// Now you can get structured data or formatted strings
	stats := client.GetStats()
	fmt.Printf("Total requests completed: %d\n", stats.TotalRequests)
	fmt.Println(stats) // Uses the String() method
}
