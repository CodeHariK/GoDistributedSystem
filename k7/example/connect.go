package main

import (
	"context"
	"fmt"
	"net/http"
	"sync"
	"time"

	"connectrpc.com/connect"
	"github.com/codeharik/k7"
	"github.com/codeharik/k7/example/api"
	"github.com/codeharik/k7/example/api/apiconnect"
	"github.com/go-resty/resty/v2"
	"golang.org/x/net/http2"
	"golang.org/x/net/http2/h2c"
)

type GreetServer struct{}

func (s *GreetServer) Greet(
	ctx context.Context,
	req *connect.Request[api.GreetRequest],
) (*connect.Response[api.GreetResponse], error) {
	res := connect.NewResponse(&api.GreetResponse{
		Greeting: fmt.Sprintf("Hello, %s!", req.Msg.Name),
	})
	res.Header().Set("Greet-Version", "v1")

	return res, nil
}

func connectServer() {
	greeter := &GreetServer{}
	mux := http.NewServeMux()
	path, handler := apiconnect.NewGreetServiceHandler(greeter)
	mux.Handle(path, handler)

	go func() {
		http.ListenAndServe(
			"localhost:8080",
			h2c.NewHandler(mux,
				&http2.Server{},
			),
		)
	}()
}

func connectAttack(restyC bool) {
	threads := 8

	var httpClient *http.Client
	if restyC {
		restyClient := resty.New().
			SetBaseURL("http://localhost:8080").
			SetTimeout(100 * time.Millisecond).
			SetRetryCount(0). // Disable retries for benchmarking
			SetCloseConnection(false).
			SetTransport(&http.Transport{
				MaxIdleConns:        100,
				MaxIdleConnsPerHost: 50,
				IdleConnTimeout:     90 * time.Second,
			})
		httpClient = restyClient.GetClient()
	} else {
		// Shared HTTP Transport for all clients
		transport := &http.Transport{
			MaxIdleConns:        100, // Max idle connections across all hosts
			MaxIdleConnsPerHost: 50,  // Increase per-host limit for connection reuse
			IdleConnTimeout:     90 * time.Second,
		}

		// Shared HTTP client with the optimized transport
		httpClient = &http.Client{
			Transport: transport,
			Timeout:   100 * time.Millisecond,
		}
	}

	// Create a pool of clients
	clients := make([]apiconnect.GreetServiceClient, 3*threads)
	for i := range clients {
		clients[i] = apiconnect.NewGreetServiceClient(
			// httpClient,
			httpClient,
			"http://localhost:8080",
			// connect.WithGRPC(),
		)
	}

	cc := 0
	var mu sync.Mutex // Ensure safe concurrent client selection

	config := k7.BenchmarkConfig{
		Threads:  threads,
		Duration: 10 * time.Second,
		AttackFunc: func() bool {
			ctx, cancel := context.WithTimeout(context.Background(), 50*time.Millisecond)
			defer cancel()

			mu.Lock()
			client := clients[cc]
			cc = (cc + 1) % len(clients)
			mu.Unlock()

			_, err := client.Greet(ctx, connect.NewRequest(&api.GreetRequest{Name: "Jane"}))
			if err != nil {
				fmt.Println(err)
				return false
			}

			return true
		},
	}
	config.Attack()
}
