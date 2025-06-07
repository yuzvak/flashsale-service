package main

import (
	"context"
	"fmt"
	"log"
	"os"
	"time"
)

func main() {
	dbConnStr := "host=localhost port=5432 user=postgres password=postgres dbname=flashsale sslmode=disable"

	config := &LoadTestConfig{
		BaseURL:             "http://localhost:8080",
		ConcurrentUsers:     400,
		TestDurationSeconds: 300,
		RampUpSeconds:       30,
	}

	if len(os.Args) > 1 {
		switch os.Args[1] {
		case "light":
			config.ConcurrentUsers = 200
			config.TestDurationSeconds = 120
			config.RampUpSeconds = 20
		case "heavy":
			config.ConcurrentUsers = 800
			config.TestDurationSeconds = 600
			config.RampUpSeconds = 60
		case "stress":
			config.ConcurrentUsers = 1500
			config.TestDurationSeconds = 900
			config.RampUpSeconds = 90
		}
	}

	if dbConn := os.Getenv("DB_CONNECTION_STRING"); dbConn != "" {
		dbConnStr = dbConn
	}

	tester, err := NewRealisticLoadTester(dbConnStr, config)
	if err != nil {
		log.Fatal("Failed to create tester:", err)
	}
	defer tester.Close()

	ctx, cancel := context.WithTimeout(context.Background(),
		time.Duration(config.TestDurationSeconds)*time.Second)
	defer cancel()

	fmt.Println("Starting realistic load test...")
	fmt.Printf("User distribution: 10%% aggressive, 60%% normal, 30%% browsers\n")
	fmt.Printf("Test will run for %d seconds with %d concurrent users\n",
		config.TestDurationSeconds, config.ConcurrentUsers)

	metrics, err := tester.RunRealisticTest(ctx)
	if err != nil {
		log.Fatal("Test failed:", err)
	}

	metrics.PrintReport()

	timestamp := time.Now().Format("20060102_150405")
	filename := fmt.Sprintf("realistic_load_test_%s.json", timestamp)
	if err := metrics.SaveToFile(filename); err != nil {
		log.Printf("Failed to save results: %v", err)
	} else {
		fmt.Printf("Results saved to: %s\n", filename)
	}
}
