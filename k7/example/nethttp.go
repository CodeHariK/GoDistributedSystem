package main

import (
	"fmt"
	"net/http"
)

func nethttpServer() {
	greet := func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("Hello world!"))
		return
	}

	go func() {
		server := http.ServeMux{}

		// Set up handlers
		server.HandleFunc("/", greet)

		// Start the server
		port := 8080
		fmt.Printf("Server running on http://localhost:%d\n", port)
		http.ListenAndServe(fmt.Sprintf(":%d", port), &server)
	}()
}
