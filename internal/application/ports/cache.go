package ports

import (
	"context"
	"time"
)

type Cache interface {
	AddItemToBloomFilter(ctx context.Context, itemID string) error
	ItemExistsInBloomFilter(ctx context.Context, itemID string) (bool, error)

	GetUserItemCount(ctx context.Context, saleID, userID string) (int, error)
	IncrementUserItemCount(ctx context.Context, saleID, userID string) error
	SetUserItemCount(ctx context.Context, saleID, userID string, count int, expiration time.Duration) error

	GetUserCheckoutCount(ctx context.Context, saleID, userID string) (int, error)
	IncrementUserCheckoutCount(ctx context.Context, saleID, userID string) error
	SetUserCheckoutCount(ctx context.Context, saleID, userID string, count int, expiration time.Duration) error
	GetAvailableCheckoutSlots(ctx context.Context, saleID, userID string, maxItems int) (int, error)

	GetUserCheckoutCode(ctx context.Context, saleID, userID string) (string, error)
	SetUserCheckoutCode(ctx context.Context, saleID, userID, code string, expiration time.Duration) error
	RemoveUserCheckoutCode(ctx context.Context, saleID, userID string) error
	SetCheckoutCode(ctx context.Context, code string, expiration time.Duration) error
	CheckoutCodeExists(ctx context.Context, code string) (bool, error)
	RemoveCheckoutCode(ctx context.Context, code string) error
	HasUserCheckedOutItem(ctx context.Context, saleID, userID, itemID string) (bool, error)
	AddUserCheckedOutItem(ctx context.Context, saleID, userID, itemID string, expiration time.Duration) error

	IncrementSaleItemsSold(ctx context.Context, saleID string, count int) error
	GetSaleItemsSold(ctx context.Context, saleID string) (int, error)
	GetSaleItemCount(ctx context.Context, saleID string) (int, error)
	IncrementCounters(ctx context.Context, saleID, userID string, itemCount int) error

	AtomicPurchaseCheck(ctx context.Context, saleID, userID string, itemCount int, maxSaleItems, maxUserItems int) (bool, error)
	AtomicUserLimitCheck(ctx context.Context, saleID, userID string, itemCount, maxItems int) (bool, error)
	AtomicSaleLimitCheck(ctx context.Context, saleID string, itemCount, maxItems int) (bool, error)
	DecrementCounters(ctx context.Context, saleID, userID string, itemCount int) error

	DistributedLock(ctx context.Context, key string, expiration time.Duration) (bool, error)
	ReleaseLock(ctx context.Context, key string) error
}
