package main

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"math/rand"
	"net/http"
	"os"
	"os/signal"
	"sort"
	"sync"
	"sync/atomic"
	"syscall"
	"time"
)

type LoadTestConfig struct {
	BaseURL             string
	ConcurrentUsers     int
	TestDurationSeconds int
	RampUpSeconds       int
	ItemCount           int
}

type TestResult struct {
	TotalRequests       int64
	SuccessfulRequests  int64
	FailedRequests      int64
	TotalCheckouts      int64
	SuccessfulPurchases int64
	FailedPurchases     int64
	ResponseTimes       []time.Duration
	Errors              map[string]int64
	mutex               sync.RWMutex
}

type PerformanceMetrics struct {
	StartTime           time.Time
	EndTime             time.Time
	TotalDuration       time.Duration
	ThroughputRPS       float64
	SuccessfulTPS       float64
	P50ResponseTime     time.Duration
	P95ResponseTime     time.Duration
	P99ResponseTime     time.Duration
	ErrorRate           float64
	CheckoutSuccessRate float64
	PurchaseSuccessRate float64
}

type LoadTester struct {
	config          *LoadTestConfig
	result          *TestResult
	client          *http.Client
	itemsCache      []string
	cacheMutex      sync.RWMutex
	lastCacheUpdate time.Time
	userPurchases   map[int]map[string]bool
	purchaseMutex   sync.RWMutex
}

type SaleResponse struct {
	ID         string `json:"id"`
	StartedAt  string `json:"started_at"`
	EndedAt    string `json:"ended_at"`
	TotalItems int    `json:"total_items"`
	ItemsSold  int    `json:"items_sold"`
	Active     bool   `json:"active"`
}

type ItemResponse struct {
	ID       string `json:"id"`
	Name     string `json:"name"`
	ImageURL string `json:"image_url"`
	Sold     bool   `json:"sold"`
}

type APIResponse struct {
	Data interface{} `json:"data"`
}

func NewLoadTester(config *LoadTestConfig) *LoadTester {
	return &LoadTester{
		config: config,
		result: &TestResult{
			ResponseTimes: make([]time.Duration, 0),
			Errors:        make(map[string]int64),
		},
		client: &http.Client{
			Timeout: 30 * time.Second,
			Transport: &http.Transport{
				MaxIdleConns:        1000,
				MaxIdleConnsPerHost: 100,
				MaxConnsPerHost:     200,
			},
		},
		itemsCache: make([]string, 0),
		userPurchases: make(map[int]map[string]bool),
	}
}

func (lt *LoadTester) recordResponse(duration time.Duration, success bool, operation string, err error) {
	lt.result.mutex.Lock()
	defer lt.result.mutex.Unlock()

	atomic.AddInt64(&lt.result.TotalRequests, 1)
	lt.result.ResponseTimes = append(lt.result.ResponseTimes, duration)

	if success {
		atomic.AddInt64(&lt.result.SuccessfulRequests, 1)
	} else {
		atomic.AddInt64(&lt.result.FailedRequests, 1)
		if err != nil {
			lt.result.Errors[fmt.Sprintf("%s: %s", operation, err.Error())]++
		}
	}
}

func (lt *LoadTester) simulateUser(ctx context.Context, userID int, wg *sync.WaitGroup) {
	defer wg.Done()

	for {
		select {
		case <-ctx.Done():
			return
		default:
			checkoutCode, checkoutSuccess := lt.performCheckouts(userID)

			if checkoutSuccess && checkoutCode != "" {
			lt.performPurchase(checkoutCode, userID)
		}

			time.Sleep(time.Duration(rand.Intn(1000)) * time.Millisecond)
		}
	}
}

func (lt *LoadTester) performCheckouts(userID int) (string, bool) {
	items, err := lt.getAvailableItems()
	if err != nil || len(items) == 0 {
		fmt.Printf("Warning: No available items found: %v\n", err)
		return "", false
	}

	lt.purchaseMutex.RLock()
	userPurchased := lt.userPurchases[userID]
	lt.purchaseMutex.RUnlock()

	availableItems := make([]string, 0, len(items))
	for _, item := range items {
		if userPurchased == nil || !userPurchased[item] {
			availableItems = append(availableItems, item)
		}
	}

	if len(availableItems) == 0 {
		return "", false
	}

	numItemsToCheckout := rand.Intn(min(5, len(availableItems))) + 1
	var checkoutCode string
	successfulCheckouts := 0

	selectedItems := make([]string, 0, numItemsToCheckout)
	usedIndices := make(map[int]bool)

	for len(selectedItems) < numItemsToCheckout {
		index := rand.Intn(len(availableItems))
		if !usedIndices[index] {
			selectedItems = append(selectedItems, availableItems[index])
			usedIndices[index] = true
		}
	}

	for _, itemID := range selectedItems {
		start := time.Now()
		url := fmt.Sprintf("%s/checkout?user_id=user_%d&item_id=%s",
			lt.config.BaseURL, userID, itemID)

		resp, err := lt.client.Post(url, "application/json", nil)
		duration := time.Since(start)

		success := false
		if err == nil && resp != nil {
			defer resp.Body.Close()

			if resp.StatusCode == http.StatusOK {
				success = true
				successfulCheckouts++
				atomic.AddInt64(&lt.result.TotalCheckouts, 1)

				var result map[string]interface{}
				body, _ := io.ReadAll(resp.Body)
				if json.Unmarshal(body, &result) == nil {
					if data, ok := result["data"].(map[string]interface{}); ok {
						if code, ok := data["code"].(string); ok {
							checkoutCode = code
						}
					}
				}
			}
		}

		lt.recordResponse(duration, success, "checkout", err)

		time.Sleep(time.Duration(rand.Intn(100)) * time.Millisecond)
	}

	return checkoutCode, successfulCheckouts > 0
}

func (lt *LoadTester) getAvailableItems() ([]string, error) {
	lt.cacheMutex.RLock()
	if time.Since(lt.lastCacheUpdate) < 30*time.Second && len(lt.itemsCache) > 0 {
		items := make([]string, len(lt.itemsCache))
		copy(items, lt.itemsCache)
		lt.cacheMutex.RUnlock()
		return items, nil
	}
	lt.cacheMutex.RUnlock()

	saleID, err := lt.getActiveSaleID()
	if err != nil {
		return nil, fmt.Errorf("failed to get active sale: %w", err)
	}

	url := fmt.Sprintf("%s/sales/%s/items", lt.config.BaseURL, saleID)
	resp, err := lt.client.Get(url)
	if err != nil {
		return nil, fmt.Errorf("failed to get items: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("API returned status %d", resp.StatusCode)
	}

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to read response: %w", err)
	}

	var apiResp APIResponse
	if err := json.Unmarshal(body, &apiResp); err != nil {
		return nil, fmt.Errorf("failed to parse response: %w", err)
	}

	itemsData, ok := apiResp.Data.([]interface{})
	if !ok {
		return nil, fmt.Errorf("unexpected response format")
	}

	availableItems := make([]string, 0)
	for _, itemData := range itemsData {
		if itemMap, ok := itemData.(map[string]interface{}); ok {
			if sold, exists := itemMap["sold"].(bool); exists && !sold {
				if id, exists := itemMap["id"].(string); exists {
					availableItems = append(availableItems, id)
				}
			}
		}
	}

	lt.cacheMutex.Lock()
	lt.itemsCache = make([]string, len(availableItems))
	copy(lt.itemsCache, availableItems)
	lt.lastCacheUpdate = time.Now()
	lt.cacheMutex.Unlock()

	return availableItems, nil
}

func (lt *LoadTester) getActiveSaleID() (string, error) {
	url := fmt.Sprintf("%s/sales/active", lt.config.BaseURL)
	resp, err := lt.client.Get(url)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("API returned status %d", resp.StatusCode)
	}

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", err
	}

	var apiResp APIResponse
	if err := json.Unmarshal(body, &apiResp); err != nil {
		return "", err
	}

	if saleData, ok := apiResp.Data.(map[string]interface{}); ok {
		if id, exists := saleData["id"].(string); exists {
			return id, nil
		}
	}

	return "", fmt.Errorf("no active sale found")
}

func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}

func (lt *LoadTester) performPurchase(checkoutCode string, userID int) {
	start := time.Now()
	url := fmt.Sprintf("%s/purchase?code=%s", lt.config.BaseURL, checkoutCode)

	resp, err := lt.client.Post(url, "application/json", nil)
	duration := time.Since(start)

	success := false
	if err == nil && resp != nil {
		defer resp.Body.Close()

		if resp.StatusCode == http.StatusOK {
			success = true
			atomic.AddInt64(&lt.result.SuccessfulPurchases, 1)

			var result map[string]interface{}
			body, _ := io.ReadAll(resp.Body)
			if json.Unmarshal(body, &result) == nil {
				if data, ok := result["data"].(map[string]interface{}); ok {
					if items, ok := data["successful_items"].([]interface{}); ok {
						lt.purchaseMutex.Lock()
						if lt.userPurchases[userID] == nil {
							lt.userPurchases[userID] = make(map[string]bool)
						}
						for _, item := range items {
							if itemStr, ok := item.(string); ok {
								lt.userPurchases[userID][itemStr] = true
							}
						}
						lt.purchaseMutex.Unlock()
					}
				}
			}
		} else {
			atomic.AddInt64(&lt.result.FailedPurchases, 1)
		}
	} else {
		atomic.AddInt64(&lt.result.FailedPurchases, 1)
	}

	lt.recordResponse(duration, success, "purchase", err)
}

func (lt *LoadTester) Run() *PerformanceMetrics {
	fmt.Printf("Starting load test with %d concurrent users for %d seconds\n",
		lt.config.ConcurrentUsers, lt.config.TestDurationSeconds)

	ctx, cancel := context.WithTimeout(context.Background(),
		time.Duration(lt.config.TestDurationSeconds)*time.Second)
	defer cancel()

	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM)
	go func() {
		<-sigChan
		fmt.Println("\nReceived interrupt signal, stopping test...")
		cancel()
	}()

	startTime := time.Now()
	var wg sync.WaitGroup

	userInterval := time.Duration(lt.config.RampUpSeconds) * time.Second / time.Duration(lt.config.ConcurrentUsers)

	for i := 0; i < lt.config.ConcurrentUsers; i++ {
		wg.Add(1)
		go lt.simulateUser(ctx, i, &wg)

		if i < lt.config.ConcurrentUsers-1 {
			time.Sleep(userInterval)
		}
	}

	go lt.monitorProgress(ctx, startTime)

	wg.Wait()
	endTime := time.Now()

	return lt.calculateMetrics(startTime, endTime)
}

func (lt *LoadTester) monitorProgress(ctx context.Context, startTime time.Time) {
	ticker := time.NewTicker(5 * time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			elapsed := time.Since(startTime)
			totalReqs := atomic.LoadInt64(&lt.result.TotalRequests)
			successReqs := atomic.LoadInt64(&lt.result.SuccessfulRequests)

			currentRPS := float64(totalReqs) / elapsed.Seconds()
			successRPS := float64(successReqs) / elapsed.Seconds()

			fmt.Printf("[%s] Total: %d, Success: %d, RPS: %.1f, Success RPS: %.1f\n",
				elapsed.Round(time.Second), totalReqs, successReqs, currentRPS, successRPS)
		}
	}
}

func (lt *LoadTester) calculateMetrics(startTime, endTime time.Time) *PerformanceMetrics {
	lt.result.mutex.RLock()
	defer lt.result.mutex.RUnlock()

	totalDuration := endTime.Sub(startTime)
	totalRequests := atomic.LoadInt64(&lt.result.TotalRequests)
	successfulRequests := atomic.LoadInt64(&lt.result.SuccessfulRequests)

	metrics := &PerformanceMetrics{
		StartTime:     startTime,
		EndTime:       endTime,
		TotalDuration: totalDuration,
	}

	if totalDuration.Seconds() > 0 {
		metrics.ThroughputRPS = float64(totalRequests) / totalDuration.Seconds()
		metrics.SuccessfulTPS = float64(successfulRequests) / totalDuration.Seconds()
	}

	if totalRequests > 0 {
		metrics.ErrorRate = float64(atomic.LoadInt64(&lt.result.FailedRequests)) / float64(totalRequests) * 100
	}

	if lt.result.TotalCheckouts > 0 {
		metrics.CheckoutSuccessRate = float64(successfulRequests) / float64(lt.result.TotalCheckouts) * 100
	}

	totalPurchaseAttempts := lt.result.SuccessfulPurchases + lt.result.FailedPurchases
	if totalPurchaseAttempts > 0 {
		metrics.PurchaseSuccessRate = float64(lt.result.SuccessfulPurchases) / float64(totalPurchaseAttempts) * 100
	}

	if len(lt.result.ResponseTimes) > 0 {
		metrics.P50ResponseTime = calculatePercentile(lt.result.ResponseTimes, 50)
		metrics.P95ResponseTime = calculatePercentile(lt.result.ResponseTimes, 95)
		metrics.P99ResponseTime = calculatePercentile(lt.result.ResponseTimes, 99)
	}

	return metrics
}

func calculatePercentile(durations []time.Duration, percentile int) time.Duration {
	if len(durations) == 0 {
		return 0
	}

	sorted := make([]time.Duration, len(durations))
	copy(sorted, durations)
	sort.Slice(sorted, func(i, j int) bool {
		return sorted[i] < sorted[j]
	})

	index := int(float64(len(sorted)) * float64(percentile) / 100.0)
	if index >= len(sorted) {
		index = len(sorted) - 1
	}
	if index < 0 {
		index = 0
	}

	return sorted[index]
}

func (pm *PerformanceMetrics) PrintReport() {
	fmt.Printf("PERFORMANCE TEST RESULTS\n")
	fmt.Printf("Test Duration: %v\n", pm.TotalDuration.Round(time.Second))
	fmt.Printf("Start Time: %s\n", pm.StartTime.Format("2006-01-02 15:04:05"))
	fmt.Printf("End Time: %s\n", pm.EndTime.Format("2006-01-02 15:04:05"))
	fmt.Printf("\n")

	fmt.Printf("THROUGHPUT METRICS:\n")
	fmt.Printf("- Total RPS: %.2f requests/second\n", pm.ThroughputRPS)
	fmt.Printf("- Successful TPS: %.2f transactions/second\n", pm.SuccessfulTPS)
	fmt.Printf("- Error Rate: %.2f%%\n", pm.ErrorRate)
	fmt.Printf("\n")

	fmt.Printf("RESPONSE TIME METRICS:\n")
	fmt.Printf("- P50 Response Time: %v\n", pm.P50ResponseTime.Round(time.Millisecond))
	fmt.Printf("- P95 Response Time: %v\n", pm.P95ResponseTime.Round(time.Millisecond))
	fmt.Printf("- P99 Response Time: %v\n", pm.P99ResponseTime.Round(time.Millisecond))
	fmt.Printf("\n")

	fmt.Printf("BUSINESS METRICS:\n")
	fmt.Printf("- Checkout Success Rate: %.2f%%\n", pm.CheckoutSuccessRate)
	fmt.Printf("- Purchase Success Rate: %.2f%%\n", pm.PurchaseSuccessRate)
	fmt.Printf("\n")
}

func (pm *PerformanceMetrics) SaveToFile(filename string) error {
	data, err := json.MarshalIndent(pm, "", "  ")
	if err != nil {
		return err
	}

	return os.WriteFile(filename, data, 0644)
}
