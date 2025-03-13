package main

import (
	"fmt"
	"log"
	"net"
	"os"
	"time"

	"github.com/hashicorp/serf/serf"
)

func main() {
	const ClusterPort = 7946

	// Get node type from CLI arguments
	nodeName := "node1"
	port := 7946
	if len(os.Args) > 1 && os.Args[1] == "node2" {
		nodeName = "node2"
		port = 7947
	}

	// Create an event channel
	eventCh := make(chan serf.Event, 256)

	// Configure Serf
	config := serf.DefaultConfig()
	config.Init()
	config.NodeName = nodeName
	ipAddr := getLocalIP()
	fmt.Printf("IP Address : %s\n", ipAddr)
	config.MemberlistConfig.BindAddr = ipAddr
	config.MemberlistConfig.BindPort = port
	config.EventCh = eventCh

	// Start Serf
	serfInstance, err := serf.Create(config)
	if err != nil {
		log.Fatalf("Failed to start Serf: %v", err)
	}
	fmt.Printf("%s started on %s:%d...\n", nodeName, config.MemberlistConfig.BindAddr, port)

	// If this is node2, try to join node1
	if nodeName == "node2" {
		_, err = serfInstance.Join([]string{fmt.Sprint(ipAddr, ":", ClusterPort)}, true) // Use real IP
		if err != nil {
			log.Printf("Failed to join cluster: %v", err)
		} else {
			fmt.Println("Node2 joined the cluster.")
		}
	}

	// Handle events
	go func() {
		for event := range eventCh {
			switch e := event.(type) {
			case serf.UserEvent:
				handleUserEvent(e, serfInstance, nodeName)
			}
		}
	}()

	// Node1 sends "ping" every 3 seconds
	if nodeName == "node1" {
		go func() {
			for {
				time.Sleep(3 * time.Second)
				fmt.Println("Node1: Sending Ping")
				serfInstance.UserEvent("ping", []byte("Hello from node1!"), true)
			}
		}()
	}

	// Keep running
	select {}
}

// Handle incoming user events
func handleUserEvent(event serf.UserEvent, serfInstance *serf.Serf, nodeName string) {
	switch event.Name {
	case "ping":
		fmt.Printf("%s: Received Ping -> %s\n", nodeName, string(event.Payload))
		if nodeName == "node2" {
			fmt.Println("Node2: Sending Pong")
			serfInstance.UserEvent("pong", []byte("Hello from node2!"), true)
		}
	case "pong":
		fmt.Printf("%s: Received Pong -> %s\n", nodeName, string(event.Payload))
	}
}

// Get local IP address
func getLocalIP() string {
	addrs, err := net.InterfaceAddrs()
	if err != nil {
		log.Fatalf("Failed to get local IP: %v", err)
	}
	for _, addr := range addrs {
		if ipNet, ok := addr.(*net.IPNet); ok && !ipNet.IP.IsLoopback() {
			if ipNet.IP.To4() != nil {
				return ipNet.IP.String()
			}
		}
	}
	return "127.0.0.1"
}
