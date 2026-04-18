package main

import (
	"bytes"
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"log"
	"math/rand"
	"net/http"
	"os/signal"
	"sync"
	"syscall"
	"time"
)

func main() {
	count := flag.Int("n", 100, "Number of endpoints to create")
	tinyPulseAPI := flag.String("api", "http://localhost:8080/api/endpoints", "TinyPulse API URL")
	pass := flag.String("password", "", "Basic auth password if enabled")
	flag.Parse()

	// 1. Start a local dummy server for the monitors to ping
	go func() {
		http.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
			time.Sleep(time.Duration(rand.Intn(50)) * time.Millisecond)
			if rand.Intn(100) < 10 {
				w.WriteHeader(http.StatusInternalServerError)
				return
			}
			w.WriteHeader(http.StatusOK)
		})
		log.Println("Dummy target server listening on :8081...")
		if err := http.ListenAndServe(":8081", nil); err != nil {
			log.Fatal(err)
		}
	}()

	time.Sleep(1 * time.Second)

	// 2. Blast the TinyPulse API to create N endpoints and track their IDs
	log.Printf("Creating %d endpoints in TinyPulse...", *count)
	
	client := &http.Client{Timeout: 5 * time.Second}
	var wg sync.WaitGroup
	var mu sync.Mutex
	var createdIDs []int64

	for i := 1; i <= *count; i++ {
		wg.Add(1)
		go func(id int) {
			defer wg.Done()

			payload := map[string]interface{}{
				"name":             fmt.Sprintf("Stress Test %03d", id),
				"url":              fmt.Sprintf("http://localhost:8081/target/%d", id),
				"interval_seconds": 10,
				"fail_threshold":   3,
			}
			body, _ := json.Marshal(payload)

			req, err := http.NewRequest(http.MethodPost, *tinyPulseAPI, bytes.NewBuffer(body))
			if err != nil {
				log.Printf("Failed to create request: %v", err)
				return
			}
			req.Header.Set("Content-Type", "application/json")
			if *pass != "" {
				req.SetBasicAuth("admin", *pass)
			}

			resp, err := client.Do(req)
			if err != nil {
				log.Printf("Failed to create endpoint %d: %v", id, err)
				return
			}
			defer resp.Body.Close()

			if resp.StatusCode == http.StatusCreated {
				// Parse the response to get the actual database ID for cleanup
				var result struct {
					ID int64 `json:"id"`
				}
				if err := json.NewDecoder(resp.Body).Decode(&result); err == nil {
					mu.Lock()
					createdIDs = append(createdIDs, result.ID)
					mu.Unlock()
				}
			} else {
				log.Printf("Unexpected status code %d for endpoint %d", resp.StatusCode, id)
			}
		}(i)

		time.Sleep(10 * time.Millisecond)
	}

	wg.Wait()
	log.Printf("Successfully created %d endpoints checking every 10 seconds!", len(createdIDs))
	log.Println("Press Ctrl+C to stop the test and automatically delete all created endpoints.")
	
	// 3. Listen for Ctrl+C to trigger cleanup
	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()

	<-ctx.Done() // Block until Ctrl+C

	log.Printf("\nShutting down... Deleting %d test endpoints...", len(createdIDs))
	
	var cleanupWg sync.WaitGroup
	for _, id := range createdIDs {
		cleanupWg.Add(1)
		go func(eid int64) {
			defer cleanupWg.Done()
			url := fmt.Sprintf("%s/%d", *tinyPulseAPI, eid)
			req, _ := http.NewRequest(http.MethodDelete, url, nil)
			if *pass != "" {
				req.SetBasicAuth("admin", *pass)
			}
			resp, err := client.Do(req)
			if err == nil {
				resp.Body.Close()
			}
		}(id)
		time.Sleep(10 * time.Millisecond) // Don't overwhelm the Delete API
	}
	
	cleanupWg.Wait()
	log.Println("Cleanup complete. Exiting.")
}
