package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
	"sync"
	"time"
)

// 👉 FIX 25: Sample Data Seeding Script
func main() {
	fmt.Println("🌱 Starting Data Seed Script...")

	apiURL := "http://localhost:3000/ingest"
	var wg sync.WaitGroup

	// Simulate 3 distinct massive failures
	components := []string{"PAYMENT_GATEWAY_01", "USER_DB_MASTER", "AUTH_SERVICE_03"}
	errors := []string{"CONNECTION_TIMEOUT", "OOM_KILLED", "LATENCY_SPIKE"}
	severities := []string{"CRITICAL", "P0", "HIGH"}

	for idx, comp := range components {
		wg.Add(1)
		go func(c, e, s string) {
			defer wg.Done()
			// Send 100 identical signals rapidly to trigger and prove the Debouncing Logic
			for i := 0; i < 100; i++ {
				payload := map[string]string{
					"component_id": c,
					"error_type":   e,
					"severity":     s,
					"message":      fmt.Sprintf("Automated load test signal #%d", i),
				}
				body, _ := json.Marshal(payload)
				http.Post(apiURL, "application/json", bytes.NewBuffer(body))
				time.Sleep(10 * time.Millisecond)
			}
			fmt.Printf("✅ Sent 100 signals for %s\n", c)
		}(comp, errors[idx], severities[idx])
	}

	wg.Wait()
	fmt.Println("🚀 Seeding complete! Check your dashboard.")
}
