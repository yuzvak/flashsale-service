package use_cases

import (
	"context"
	"fmt"
	"time"

	"github.com/yuzvak/flashsale-service/internal/application/ports"
	"github.com/yuzvak/flashsale-service/internal/domain/errors"
	"github.com/yuzvak/flashsale-service/internal/domain/sale"
	"github.com/yuzvak/flashsale-service/internal/pkg/logger"
)

type PurchaseUseCase struct {
	saleRepo     ports.SaleRepository
	checkoutRepo ports.CheckoutRepository
	cache        ports.Cache
	purchaseSvc  *sale.PurchaseService
	log          *logger.Logger

	maxItemsPerSale int
	maxItemsPerUser int
	retryAttempts   int
	lockTimeout     time.Duration
}

func NewPurchaseUseCase(
	saleRepo ports.SaleRepository,
	checkoutRepo ports.CheckoutRepository,
	cache ports.Cache,
	log *logger.Logger,
) *PurchaseUseCase {
	return &PurchaseUseCase{
		saleRepo:        saleRepo,
		checkoutRepo:    checkoutRepo,
		cache:           cache,
		purchaseSvc:     sale.NewPurchaseService(10000, 10),
		log:             log,
		maxItemsPerSale: 10000,
		maxItemsPerUser: 10,
		retryAttempts:   2,
		lockTimeout:     time.Second * 3,
	}
}

func (uc *PurchaseUseCase) ExecutePurchase(ctx context.Context, checkoutCode string) (*sale.PurchaseResult, error) {
	exists, err := uc.cache.CheckoutCodeExists(ctx, checkoutCode)
	if err != nil {
		uc.log.Error("Failed to check checkout code", "error", err, "checkout_code", checkoutCode)
		return nil, err
	}

	checkout, err := uc.checkoutRepo.GetCheckoutByCode(ctx, checkoutCode)
	if err != nil {
		uc.log.Error("Failed to get checkout", "error", err, "checkout_code", checkoutCode)
		return nil, errors.ErrCheckoutNotFound
	}

	if !exists {
		if err := uc.cache.SetCheckoutCode(ctx, checkoutCode, time.Hour); err != nil {
			uc.log.Warn("Failed to restore checkout code in cache", "error", err, "checkout_code", checkoutCode)
		}
	}

	lockKey := fmt.Sprintf("purchase:%s", checkoutCode)
	locked, err := uc.cache.DistributedLock(ctx, lockKey, uc.lockTimeout)
	if err != nil {
		uc.log.Error("Failed to acquire lock", "error", err, "lock_key", lockKey)
		return nil, fmt.Errorf("failed to acquire lock: %w", err)
	}
	if !locked {
		return nil, fmt.Errorf("another purchase is in progress for this user")
	}
	defer func() {
		if err := uc.cache.ReleaseLock(ctx, lockKey); err != nil {
			uc.log.Error("Failed to release lock", "error", err, "lock_key", lockKey)
		}
	}()

	var result *sale.PurchaseResult
	for attempt := 0; attempt < uc.retryAttempts; attempt++ {
		result, err = uc.attemptPurchase(ctx, checkout)
		if err == nil {
			break
		}

		uc.log.Warn("Purchase attempt failed", "attempt", attempt+1, "error", err.Error(), "checkout_code", checkoutCode)

		if isBusinessLogicError(err) {
			break
		}

		if attempt < uc.retryAttempts-1 {
			time.Sleep(time.Millisecond * time.Duration(100*(attempt+1)))
		}
	}

	if err != nil {
		return nil, err
	}

	if err := uc.cleanupCheckout(ctx, checkoutCode, checkout.SaleID, checkout.UserID); err != nil {
		uc.log.Error("Failed to cleanup checkout", "error", err, "checkout_code", checkoutCode)
	}

	return result, nil
}

func (uc *PurchaseUseCase) attemptPurchase(ctx context.Context, checkout *sale.Checkout) (*sale.PurchaseResult, error) {
	for _, itemID := range checkout.ItemIDs {
		if err := uc.checkoutRepo.LogCheckoutAttempt(ctx, checkout.SaleID, checkout.UserID, checkout.Code, itemID); err != nil {
			uc.log.Error("Failed to log checkout attempt", "error", err, "checkout_code", checkout.Code, "item_id", itemID)
		}
	}

	currentUserCount, _ := uc.cache.GetUserItemCount(ctx, checkout.SaleID, checkout.UserID)
	uc.log.Info("Pre-purchase check",
		"user_id", checkout.UserID,
		"sale_id", checkout.SaleID,
		"current_user_count", currentUserCount,
		"item_count", len(checkout.ItemIDs),
		"max_sale_items", uc.maxItemsPerSale,
		"max_user_items", uc.maxItemsPerUser)

	currentSaleCount, _ := uc.cache.GetSaleItemCount(ctx, checkout.SaleID)
	if currentSaleCount+len(checkout.ItemIDs) > uc.maxItemsPerSale {
		uc.log.Warn("Sale limit would be exceeded",
			"sale_id", checkout.SaleID,
			"current_sale_count", currentSaleCount,
			"item_count", len(checkout.ItemIDs),
			"max_sale_items", uc.maxItemsPerSale)
		return nil, errors.ErrSaleLimitExceeded
	}

	if currentUserCount+len(checkout.ItemIDs) > uc.maxItemsPerUser {
		uc.log.Warn("User limit would be exceeded",
			"user_id", checkout.UserID,
			"sale_id", checkout.SaleID,
			"current_user_count", currentUserCount,
			"item_count", len(checkout.ItemIDs),
			"max_user_items", uc.maxItemsPerUser)
		return nil, errors.ErrUserLimitExceeded
	}

	txRepo, err := uc.saleRepo.BeginTx(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to begin transaction: %w", err)
	}
	defer func() {
		if err != nil {
			_ = txRepo.RollbackTx(ctx)
		}
	}()

	existingResult, err := txRepo.GetPurchaseResult(ctx, checkout.Code)
	if err != nil {
		uc.log.Error("Failed to check existing purchase result", "error", err, "checkout_code", checkout.Code)
		return nil, err
	}
	if existingResult != nil {
		return nil, errors.ErrCheckoutAlreadyProcessed
	}

	saleEntity, err := txRepo.GetSaleByID(ctx, checkout.SaleID)
	if err != nil {
		return nil, fmt.Errorf("failed to get sale: %w", err)
	}

	items := make([]*sale.Item, 0, len(checkout.ItemIDs))
	for _, itemID := range checkout.ItemIDs {
		item, err := txRepo.GetItemByID(ctx, itemID)
		if err != nil {
			uc.log.Error("Failed to get item", "error", err, "item_id", itemID)
			continue
		}
		items = append(items, item)
	}

	if len(items) == 0 {
		return nil, fmt.Errorf("no valid items found")
	}

	userLimits := &sale.UserLimits{
		CurrentItemCount: 0, // Will be checked atomically
		MaxItemsPerUser:  uc.maxItemsPerUser,
	}

	if err := uc.purchaseSvc.ValidatePurchase(saleEntity, userLimits, items); err != nil {
		return nil, fmt.Errorf("purchase validation failed: %w", err)
	}

	successfulPurchases := make([]string, 0, len(items))
	for _, item := range items {
		alreadySold, err := uc.cache.ItemExistsInBloomFilter(ctx, item.ID)
		if err != nil {
			uc.log.Error("Bloom filter check failed", "error", err, "item_id", item.ID)
		}
		if alreadySold {
			uc.log.Info("Item likely already sold (bloom filter)", "item_id", item.ID)
			continue
		}

		success, err := txRepo.MarkItemAsSold(ctx, item.ID, checkout.UserID)
		if err != nil {
			uc.log.Error("Failed to mark item as sold", "error", err, "item_id", item.ID)
			continue
		}

		if success {
			successfulPurchases = append(successfulPurchases, item.ID)
			_ = uc.cache.AddItemToBloomFilter(ctx, item.ID)
		} else {
			_ = uc.cache.AddItemToBloomFilter(ctx, item.ID)
		}
	}

	result := uc.purchaseSvc.CalculatePurchaseResult(items, successfulPurchases)

	if len(successfulPurchases) > 0 {
		if err := uc.cache.IncrementCounters(ctx, checkout.SaleID, checkout.UserID, len(successfulPurchases)); err != nil {
			uc.log.Error("Failed to increment counters", "error", err, "checkout_code", checkout.Code, "increment", len(successfulPurchases))
		}
	}

	if len(successfulPurchases) > 0 {
		saleEntity.ItemsSold += len(successfulPurchases)
		if err := txRepo.UpdateSale(ctx, saleEntity); err != nil {
			return nil, fmt.Errorf("failed to update sale: %w", err)
		}
	}

	if err := txRepo.SavePurchaseResult(ctx, checkout.Code, result); err != nil {
		return nil, fmt.Errorf("failed to save purchase result: %w", err)
	}

	if err := txRepo.CommitTx(ctx); err != nil {
		return nil, fmt.Errorf("failed to commit transaction: %w", err)
	}

	if len(successfulPurchases) == 0 {
		return nil, errors.ErrAllItemsSold
	}

	uc.log.Info("Purchase completed",
		"checkout_code", checkout.Code,
		"user_id", checkout.UserID,
		"sale_id", checkout.SaleID,
		"attempted", len(items),
		"successful", len(successfulPurchases),
	)

	return result, nil
}

func (uc *PurchaseUseCase) cleanupCheckout(ctx context.Context, checkoutCode, saleID, userID string) error {
	if err := uc.cache.RemoveUserCheckoutCode(ctx, saleID, userID); err != nil {
		uc.log.Error("Failed to remove user checkout code from cache", "error", err)
	}

	if err := uc.cache.RemoveCheckoutCode(ctx, checkoutCode); err != nil {
		uc.log.Error("Failed to remove checkout from cache", "error", err)
	}

	if err := uc.checkoutRepo.DeleteCheckout(ctx, checkoutCode); err != nil {
		uc.log.Error("Failed to delete checkout from database", "error", err)
		return err
	}

	return nil
}

func isBusinessLogicError(err error) bool {
	switch err {
	case errors.ErrCheckoutNotFound, errors.ErrSaleNotFound, errors.ErrUserLimitExceeded, errors.ErrCheckoutAlreadyProcessed:
		return true
	default:
		return false
	}
}
