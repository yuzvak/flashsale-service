package sale

import (
	"errors"
	"time"

	domainErrors "github.com/yuzvak/flashsale-service/internal/domain/errors"
)

type UserLimits struct {
	CurrentItemCount int
	MaxItemsPerUser  int
}

type PurchaseService struct {
	maxItemsPerSale int
	maxItemsPerUser int
}

func NewPurchaseService(maxItemsPerSale, maxItemsPerUser int) *PurchaseService {
	return &PurchaseService{
		maxItemsPerSale: maxItemsPerSale,
		maxItemsPerUser: maxItemsPerUser,
	}
}

func (s *PurchaseService) ValidatePurchase(sale *Sale, userLimits *UserLimits, items []*Item) error {
	if sale == nil {
		return errors.New("sale cannot be nil")
	}

	if !sale.IsActive(time.Now().UTC()) {
		return domainErrors.ErrSaleNotActive
	}

	if len(items) == 0 {
		return domainErrors.ErrNoItemsToPurchase
	}

	if sale.ItemsSold+len(items) > s.maxItemsPerSale {
		return domainErrors.ErrSaleLimitExceeded
	}

	if userLimits.CurrentItemCount+len(items) > s.maxItemsPerUser {
		return domainErrors.ErrUserLimitExceeded
	}

	for _, item := range items {
		if !item.BelongsToSale(sale.ID) {
			return domainErrors.ErrItemNotInSale
		}
	}

	return nil
}

func (s *PurchaseService) CalculatePurchaseResult(attemptedItems []*Item, successfulPurchases []string) *PurchaseResult {
	result := &PurchaseResult{
		Items:          make([]PurchaseItemResult, 0, len(attemptedItems)),
		TotalPurchased: len(successfulPurchases),
		FailedCount:    len(attemptedItems) - len(successfulPurchases),
		Success:        len(successfulPurchases) > 0,
	}

	successMap := make(map[string]bool)
	for _, itemID := range successfulPurchases {
		successMap[itemID] = true
	}

	for _, item := range attemptedItems {
		result.Items = append(result.Items, PurchaseItemResult{
			ID:   item.ID,
			Name: item.Name,
			Sold: successMap[item.ID],
		})
	}

	return result
}

type PurchaseResult struct {
	Success        bool                 `json:"success"`
	Items          []PurchaseItemResult `json:"purchased_items"`
	TotalPurchased int                  `json:"total_purchased"`
	FailedCount    int                  `json:"failed_count"`
}

type PurchaseItemResult struct {
	ID   string `json:"id"`
	Name string `json:"name"`
	Sold bool   `json:"sold"`
}
