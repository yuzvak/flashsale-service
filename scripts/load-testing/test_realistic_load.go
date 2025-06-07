package main

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"math/rand"
	"sync"
	"sync/atomic"
	"time"

	_ "github.com/lib/pq"
)

type RealisticLoadTester struct {
	db              *sql.DB
	config          *LoadTestConfig
	httpTester      *LoadTester
	itemDistributor *ItemDistributor
	userCheckouts   map[int]int
	userMutex       sync.RWMutex
	userPurchases   map[int]map[string]bool
	purchaseMutex   sync.RWMutex
}

type ItemDistributor struct {
	allItems     []string
	popularItems []string
	normalItems  []string
	mutex        sync.RWMutex
	lastUpdate   time.Time
}

type UserBehaviorProfile struct {
	Name                string
	CheckoutProbability float64
	PurchaseProbability float64
	ItemsPerSession     int
	SessionDelay        time.Duration
	PopularItemBias     float64
}

var UserProfiles = []UserBehaviorProfile{
	{
		Name:                "aggressive_buyer",
		CheckoutProbability: 0.9,
		PurchaseProbability: 0.8,
		ItemsPerSession:     3,
		SessionDelay:        100 * time.Millisecond,
		PopularItemBias:     0.7,
	},
	{
		Name:                "normal_buyer",
		CheckoutProbability: 0.6,
		PurchaseProbability: 0.4,
		ItemsPerSession:     2,
		SessionDelay:        300 * time.Millisecond,
		PopularItemBias:     0.3,
	},
	{
		Name:                "browser",
		CheckoutProbability: 0.3,
		PurchaseProbability: 0.1,
		ItemsPerSession:     1,
		SessionDelay:        800 * time.Millisecond,
		PopularItemBias:     0.1,
	},
}

func NewRealisticLoadTester(dbConnStr string, config *LoadTestConfig) (*RealisticLoadTester, error) {
	db, err := sql.Open("postgres", dbConnStr)
	if err != nil {
		return nil, err
	}

	if err := db.Ping(); err != nil {
		return nil, err
	}

	httpTester := NewLoadTester(config)

	return &RealisticLoadTester{
		db:              db,
		config:          config,
		httpTester:      httpTester,
		itemDistributor: &ItemDistributor{},
		userCheckouts:   make(map[int]int),
		userPurchases:   make(map[int]map[string]bool),
	}, nil
}

func (rlt *RealisticLoadTester) LoadItemsFromDB() error {
	var saleID string
	err := rlt.db.QueryRow(`
		SELECT id FROM sales 
		WHERE started_at <= NOW() AND ended_at > NOW() 
		ORDER BY started_at DESC LIMIT 1
	`).Scan(&saleID)

	if err != nil {
		return fmt.Errorf("no active sale found: %w", err)
	}

	rows, err := rlt.db.Query(`
		SELECT id FROM items 
		WHERE sale_id = $1 AND sold = FALSE 
		ORDER BY created_at
	`, saleID)

	if err != nil {
		return err
	}
	defer rows.Close()

	var allItems []string
	for rows.Next() {
		var itemID string
		if err := rows.Scan(&itemID); err != nil {
			return err
		}
		allItems = append(allItems, itemID)
	}

	if len(allItems) == 0 {
		return fmt.Errorf("no available items found")
	}

	popularCount := len(allItems) / 5

	rand.Shuffle(len(allItems), func(i, j int) {
		allItems[i], allItems[j] = allItems[j], allItems[i]
	})

	rlt.itemDistributor.mutex.Lock()
	defer rlt.itemDistributor.mutex.Unlock()

	rlt.itemDistributor.allItems = allItems
	rlt.itemDistributor.popularItems = allItems[:popularCount]
	rlt.itemDistributor.normalItems = allItems[popularCount:]
	rlt.itemDistributor.lastUpdate = time.Now()

	fmt.Printf("Loaded %d items (%d popular, %d normal) from sale %s\n",
		len(allItems), len(rlt.itemDistributor.popularItems),
		len(rlt.itemDistributor.normalItems), saleID)

	return nil
}

func (rlt *RealisticLoadTester) RunRealisticTest(ctx context.Context) (*PerformanceMetrics, error) {
	if err := rlt.LoadItemsFromDB(); err != nil {
		return nil, fmt.Errorf("failed to load items: %w", err)
	}

	go rlt.periodicItemUpdate(ctx)

	fmt.Printf("Starting realistic load test with %d concurrent users\n", rlt.config.ConcurrentUsers)

	var wg sync.WaitGroup
	startTime := time.Now()

	userProfiles := rlt.distributeUserProfiles()

	for userID := 0; userID < rlt.config.ConcurrentUsers; userID++ {
		wg.Add(1)
		profile := userProfiles[userID]

		go rlt.simulateRealisticUser(ctx, userID, profile, &wg)

		time.Sleep(time.Duration(rlt.config.RampUpSeconds*1000/rlt.config.ConcurrentUsers) * time.Millisecond)
	}

	go rlt.monitorProgress(ctx, startTime)

	done := make(chan struct{})
	go func() {
		defer close(done)
		wg.Wait()
	}()

	select {
	case <-ctx.Done():
		fmt.Println("Test cancelled by context")
	case <-done:
		fmt.Println("Test completed")
	}

	endTime := time.Now()
	return rlt.httpTester.calculateMetrics(startTime, endTime), nil
}

func (rlt *RealisticLoadTester) distributeUserProfiles() []UserBehaviorProfile {
	profiles := make([]UserBehaviorProfile, rlt.config.ConcurrentUsers)

	aggressiveCount := rlt.config.ConcurrentUsers / 10
	normalCount := rlt.config.ConcurrentUsers * 6 / 10

	index := 0

	for i := 0; i < aggressiveCount && index < len(profiles); i++ {
		profiles[index] = UserProfiles[0]
		index++
	}

	for i := 0; i < normalCount && index < len(profiles); i++ {
		profiles[index] = UserProfiles[1]
		index++
	}

	for index < len(profiles) {
		profiles[index] = UserProfiles[2]
		index++
	}

	rand.Shuffle(len(profiles), func(i, j int) {
		profiles[i], profiles[j] = profiles[j], profiles[i]
	})

	return profiles
}

func (rlt *RealisticLoadTester) simulateRealisticUser(ctx context.Context, userID int, profile UserBehaviorProfile, wg *sync.WaitGroup) {
	defer wg.Done()

	for {
		select {
		case <-ctx.Done():
			return
		default:
			if rand.Float64() > profile.CheckoutProbability {
				time.Sleep(profile.SessionDelay)
				continue
			}

			if rlt.getUserCheckoutCount(userID) >= 10 {
				time.Sleep(profile.SessionDelay)
				continue
			}

			items := rlt.selectItemsForUser(userID, profile)
			if len(items) == 0 {
				time.Sleep(profile.SessionDelay)
				continue
			}

			checkoutCodes := rlt.performRealisticCheckout(userID, items, profile)

			for _, checkoutCode := range checkoutCodes {
			if checkoutCode != "" {
				purchaseRoll := rand.Float64()
				if purchaseRoll <= profile.PurchaseProbability {
				time.Sleep(time.Duration(rand.Intn(1000)+200) * time.Millisecond)
				rlt.performRealisticPurchase(checkoutCode, userID)
			}
			}
		}

			time.Sleep(profile.SessionDelay)
		}
	}
}

func (rlt *RealisticLoadTester) selectItemsForUser(userID int, profile UserBehaviorProfile) []string {
	rlt.itemDistributor.mutex.RLock()
	allItems := rlt.itemDistributor.allItems
	popularItems := rlt.itemDistributor.popularItems
	normalItems := rlt.itemDistributor.normalItems
	rlt.itemDistributor.mutex.RUnlock()

	if len(allItems) == 0 {
		return nil
	}

	rlt.purchaseMutex.RLock()
	userPurchased := rlt.userPurchases[userID]
	rlt.purchaseMutex.RUnlock()

	availablePopular := make([]string, 0)
	availableNormal := make([]string, 0)

	for _, item := range popularItems {
		if userPurchased == nil || !userPurchased[item] {
			availablePopular = append(availablePopular, item)
		}
	}

	for _, item := range normalItems {
		if userPurchased == nil || !userPurchased[item] {
			availableNormal = append(availableNormal, item)
		}
	}

	if len(availablePopular) == 0 && len(availableNormal) == 0 {
		return nil
	}

	maxItems := min(profile.ItemsPerSession, len(availablePopular)+len(availableNormal))
	numItems := rand.Intn(maxItems) + 1

	selectedItems := make([]string, 0, numItems)
	usedItems := make(map[string]bool)

	for len(selectedItems) < numItems {
		var item string

		if rand.Float64() < profile.PopularItemBias && len(availablePopular) > 0 {
			index := rand.Intn(len(availablePopular))
			item = availablePopular[index]
		} else if len(availableNormal) > 0 {
			index := rand.Intn(len(availableNormal))
			item = availableNormal[index]
		}

		if item != "" && !usedItems[item] {
			selectedItems = append(selectedItems, item)
			usedItems[item] = true
		} else if item == "" {
			break
		}
	}

	return selectedItems
}

func (rlt *RealisticLoadTester) performRealisticCheckout(userID int, items []string, profile UserBehaviorProfile) []string {
	var checkoutCodes []string

	for _, itemID := range items {
		if rlt.getUserCheckoutCount(userID) >= 10 {
			break
		}

		start := time.Now()

		url := fmt.Sprintf("%s/checkout?user_id=user_%d&item_id=%s",
			rlt.config.BaseURL, userID, itemID)

		resp, err := rlt.httpTester.client.Post(url, "application/json", nil)
		duration := time.Since(start)

		success := false
		if err == nil && resp != nil {
			defer resp.Body.Close()

			if resp.StatusCode == 200 {
				success = true
				atomic.AddInt64(&rlt.httpTester.result.TotalCheckouts, 1)
				rlt.incrementUserCheckoutCount(userID)

				var result map[string]interface{}
				body, _ := io.ReadAll(resp.Body)
				if json.Unmarshal(body, &result) == nil {
					if code, ok := result["code"].(string); ok {
						checkoutCodes = append(checkoutCodes, code)
					}
				}
			}
		}

		rlt.httpTester.recordResponse(duration, success, "checkout", err)

		time.Sleep(time.Duration(rand.Intn(100)+50) * time.Millisecond)
	}

	return checkoutCodes
}

func (rlt *RealisticLoadTester) performRealisticPurchase(checkoutCode string, userID int) {
	start := time.Now()
	url := fmt.Sprintf("%s/purchase?code=%s", rlt.config.BaseURL, checkoutCode)

	resp, err := rlt.httpTester.client.Post(url, "application/json", nil)
	duration := time.Since(start)

	success := false
	if err == nil && resp != nil {
		defer resp.Body.Close()

		if resp.StatusCode == 200 {
			success = true
			atomic.AddInt64(&rlt.httpTester.result.SuccessfulPurchases, 1)

			var result map[string]interface{}
			body, _ := io.ReadAll(resp.Body)
			if json.Unmarshal(body, &result) == nil {
				if data, ok := result["data"].(map[string]interface{}); ok {
					if items, ok := data["successful_items"].([]interface{}); ok {
						rlt.purchaseMutex.Lock()
						if rlt.userPurchases[userID] == nil {
							rlt.userPurchases[userID] = make(map[string]bool)
						}
						for _, item := range items {
							if itemStr, ok := item.(string); ok {
								rlt.userPurchases[userID][itemStr] = true
							}
						}
						rlt.purchaseMutex.Unlock()
					}
				}
			}
		} else {
			atomic.AddInt64(&rlt.httpTester.result.FailedPurchases, 1)
		}
	} else {
		atomic.AddInt64(&rlt.httpTester.result.FailedPurchases, 1)
	}

	rlt.httpTester.recordResponse(duration, success, "purchase", err)
}

func (rlt *RealisticLoadTester) periodicItemUpdate(ctx context.Context) {
	ticker := time.NewTicker(2 * time.Minute)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			if err := rlt.LoadItemsFromDB(); err != nil {
				log.Printf("Failed to update items: %v", err)
			}
		}
	}
}

func (rlt *RealisticLoadTester) monitorProgress(ctx context.Context, startTime time.Time) {
	ticker := time.NewTicker(10 * time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			elapsed := time.Since(startTime)
			totalReqs := atomic.LoadInt64(&rlt.httpTester.result.TotalRequests)
			successReqs := atomic.LoadInt64(&rlt.httpTester.result.SuccessfulRequests)
			checkouts := atomic.LoadInt64(&rlt.httpTester.result.TotalCheckouts)
			purchases := atomic.LoadInt64(&rlt.httpTester.result.SuccessfulPurchases)

			currentRPS := float64(totalReqs) / elapsed.Seconds()
			successRPS := float64(successReqs) / elapsed.Seconds()

			rlt.itemDistributor.mutex.RLock()
			availableItems := len(rlt.itemDistributor.allItems)
			rlt.itemDistributor.mutex.RUnlock()

			rlt.userMutex.RLock()
			usersAtLimit := 0
			totalUserCheckouts := 0
			for _, count := range rlt.userCheckouts {
				totalUserCheckouts += count
				if count >= 10 {
					usersAtLimit++
				}
			}
			rlt.userMutex.RUnlock()

			fmt.Printf("[%s] Requests: %d (%.1f RPS), Success: %d (%.1f RPS), Checkouts: %d, Purchases: %d, Available Items: %d, Users at limit: %d\n",
				elapsed.Round(time.Second), totalReqs, currentRPS, successReqs, successRPS, checkouts, purchases, availableItems, usersAtLimit)
		}
	}
}

func (rlt *RealisticLoadTester) getUserCheckoutCount(userID int) int {
	rlt.userMutex.RLock()
	defer rlt.userMutex.RUnlock()
	return rlt.userCheckouts[userID]
}

func (rlt *RealisticLoadTester) incrementUserCheckoutCount(userID int) {
	rlt.userMutex.Lock()
	defer rlt.userMutex.Unlock()
	rlt.userCheckouts[userID]++
}

func (rlt *RealisticLoadTester) Close() error {
	return rlt.db.Close()
}
