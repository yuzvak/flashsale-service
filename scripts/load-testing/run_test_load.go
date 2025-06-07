package loadtest

import (
	"fmt"
	"log"
	"os"
	"time"
)

func main() {
	config := &LoadTestConfig{
		BaseURL:             "http://localhost:8080",
		ConcurrentUsers:     100,
		TestDurationSeconds: 60,
		RampUpSeconds:       10,
		ItemCount:           10000,
	}

	if len(os.Args) > 1 {
		switch os.Args[1] {
		case "light":
			config.ConcurrentUsers = 50
			config.TestDurationSeconds = 30
		case "heavy":
			config.ConcurrentUsers = 500
			config.TestDurationSeconds = 300
		case "stress":
			config.ConcurrentUsers = 1000
			config.TestDurationSeconds = 600
		}
	}

	loadTester := loadtest.NewLoadTester(config)

	fmt.Printf("Configuration:\n")
	fmt.Printf("- Base URL: %s\n", config.BaseURL)
	fmt.Printf("- Concurrent Users: %d\n", config.ConcurrentUsers)
	fmt.Printf("- Test Duration: %d seconds\n", config.TestDurationSeconds)
	fmt.Printf("- Ramp Up: %d seconds\n", config.RampUpSeconds)
	fmt.Printf("\nStarting test...\n\n")

	metrics := loadTester.Run()

	metrics.PrintReport()

	timestamp := time.Now().Format("20060102_150405")
	filename := fmt.Sprintf("load_test_results_%s.json", timestamp)
	if err := metrics.SaveToFile(filename); err != nil {
		log.Printf("Failed to save results to file: %v", err)
	} else {
		fmt.Printf("Results saved to: %s\n", filename)
	}
}
