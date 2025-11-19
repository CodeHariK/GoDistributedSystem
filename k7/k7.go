package k7

import (
	"encoding/json"
	"fmt"
	"net/http"
	"sync"
	"time"

	"github.com/shirou/gopsutil/cpu"
)

// Bucket stores per-100ms stats
type Bucket struct {
	Requests     int
	Success      int
	TotalLatency time.Duration
	CPUUsage     float64
	// MemUsage     float64
	// NetSent      uint64
	// NetRecv      uint64
}

// BenchmarkConfig defines the test parameters
type BenchmarkConfig struct {
	Threads    int
	Duration   time.Duration
	AttackFunc func() bool
}

// RunBenchmark executes the test
func (config BenchmarkConfig) RunBenchmark() []Bucket {
	var wg sync.WaitGroup

	if config.Threads == 0 {
		config.Threads = 1
	}

	numbuckets := int(config.Duration.Milliseconds() / 100)

	startTime := time.Now()

	// Buckets to store per-100Millisecond stats
	buckets := make([]Bucket, numbuckets+1)
	var bucketMutex sync.Mutex

	fmt.Println("Starting benchmark...")

	// Start request workers
	for thread := 0; thread < config.Threads; thread++ {
		wg.Add(1)
		go func() {
			defer wg.Done()

			endTime := time.Now().Add(config.Duration)

			bucketFill := 1
			reqPerBucketThread := 0

			for time.Now().Before(endTime) {
				start := time.Now()

				timeElapsed := int(time.Since(startTime).Milliseconds()) / 100

				success := config.AttackFunc()

				reqPerBucketThread++

				duration := time.Since(start)

				if timeElapsed < len(buckets) {
					bucketMutex.Lock()
					if success {
						buckets[timeElapsed].Success++
					}
					buckets[timeElapsed].Requests++

					buckets[timeElapsed].TotalLatency += duration

					if bucketFill == timeElapsed {

						// Get system metrics
						cpuPercent, _ := cpu.Percent(0, false)
						// memStats, _ := mem.VirtualMemory()
						// netStats, _ := net.IOCounters(false)

						// Store in bucket
						buckets[timeElapsed].CPUUsage = cpuPercent[0]
						// buckets[timeElapsed].MemUsage = memStats.UsedPercent
						// buckets[timeElapsed].NetSent = netStats[0].BytesSent
						// buckets[timeElapsed].NetRecv = netStats[0].BytesRecv

						if thread == 0 {
							// data, _ := json.Marshal(buckets[timeElapsed-1])
							// fmt.Println(string(data))
							broadcastResults(buckets[timeElapsed-1])
						}

						bucketFill++

					}

					bucketMutex.Unlock()
				}
			}
		}()
	}

	wg.Wait()

	data, err := json.MarshalIndent(buckets[:numbuckets], "", " ")
	if err != nil {
		fmt.Println("Error encoding JSON:", err)
	}
	fmt.Println(string(data))

	return buckets[:numbuckets]
}

func (config BenchmarkConfig) Attack() {
	server := http.ServeMux{}

	server.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		http.ServeFile(w, r, "k7.html")
	})
	server.HandleFunc("/ws", config.serveWebSocket)

	// Start the server
	port := 8888
	fmt.Printf("Server running on http://localhost:%d\n", port)
	http.ListenAndServe(fmt.Sprintf(":%d", port), &server)
}
