package main

import (
	"flag"

	"github.com/codeharik/gossip"
)

func main() {
	port := flag.Int("port", 8080, "Port to run the gossip node on")
	flag.Parse() // Parse command-line arguments

	gossip.Gossip(*port)
}
