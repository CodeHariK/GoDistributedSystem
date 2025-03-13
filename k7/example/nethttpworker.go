package main

import (
	"fmt"
	"io/fs"
	"io/ioutil"
	"net/http"
	"path/filepath"
	"strings"
	"sync"
)

// CPU-heavy task: Compute Fibonacci
func fib(n int) int {
	if n <= 1 {
		return n
	}
	return fib(n-1) + fib(n-2)
}

// Recursively reads files and counts letters
func countLettersInDir(dir string) (int, error) {
	totalLetters := 0

	err := filepath.WalkDir(dir, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() {
			return nil
		}

		content, err := ioutil.ReadFile(path)
		if err != nil {
			return err
		}

		totalLetters += len(strings.ReplaceAll(string(content), " ", "")) // Count non-space characters
		return nil
	})

	return totalLetters, err
}

// Handler that spawns goroutines for concurrent processing
func handler(w http.ResponseWriter, r *http.Request) {
	var wg sync.WaitGroup

	wg.Add(5)
	for i := 0; i < 5; i++ {
		go func() {
			defer wg.Done()

			fib(20) // Simulate CPU-heavy task

			dir := "example/api" // Change this to your test directory

			// Recursively read directory
			count, err := countLettersInDir(dir)
			if err != nil {
				fmt.Println(err)
				http.Error(w, "Wrong", http.StatusConflict)
				return
			}

			fmt.Println(count)
		}()
	}
	wg.Wait()

	// fmt.Println("Done")

	w.WriteHeader(http.StatusOK)
	w.Write([]byte("Hello world!"))
	return
}

func nethttpWorkerServer() {
	go func() {
		server := http.ServeMux{}

		// Set up handlers
		server.HandleFunc("/", handler)

		// Start the server
		port := 8080
		fmt.Printf("Server running on http://localhost:%d\n", port)
		http.ListenAndServe(fmt.Sprintf(":%d", port), &server)
	}()

	restyAttack()
}
