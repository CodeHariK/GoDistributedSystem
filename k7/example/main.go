package main

import (
	"fmt"
	"runtime"
	"time"

	"github.com/codeharik/k7"
	"github.com/go-resty/resty/v2"
)

var (
	restyClients  = []resty.Client{}
	restyClientId = 0
)

func restyAttack() {
	threads := 8

	for i := 0; i < 3*threads; i++ {
		restyClients = append(restyClients, *resty.New())
	}

	config := k7.BenchmarkConfig{
		Threads:  threads,
		Duration: 10 * time.Second,
		AttackFunc: func() bool {
			res, err := restyClients[restyClientId].R().Get("http://localhost:8080")
			if err != nil {
				fmt.Println(err)
				return false
			}

			// fmt.Println(string(res.Body()), res.StatusCode() == 200)

			restyClientId = (restyClientId + 1) % len(restyClients)

			return res.StatusCode() == 200
		},
	}
	config.Attack()
}

type Mode int

const (
	Connect Mode = iota
	Fiber
	NetHTTP
	NetHTTPWorker
)

func main() {
	fmt.Println("Number of CPU", runtime.NumCPU(), runtime.NumCgoCall(), runtime.NumGoroutine())
	runtime.GOMAXPROCS(runtime.NumCPU())

	mode := Connect

	switch mode {
	case Connect:
		resty := false
		fmt.Println("Starting Connect Server, Resty:", resty)
		connectServer()
		connectAttack(resty)
	case Fiber:
		fmt.Println("Starting Fiber Server")
		fiberServer()
	case NetHTTP:
		fmt.Println("Starting NetHTTP Server")
		nethttpServer()
	case NetHTTPWorker:
		fmt.Println("Starting NetHTTPWorker Server")
		nethttpWorkerServer()
	default:
		fmt.Println("Invalid mode")
	}

	restyAttack()
}
