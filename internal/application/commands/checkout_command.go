package commands

import (
	"context"
	"time"

	"github.com/yuzvak/flashsale-service/internal/application/ports"
	"github.com/yuzvak/flashsale-service/internal/domain/errors"
	"github.com/yuzvak/flashsale-service/internal/domain/sale"
	"github.com/yuzvak/flashsale-service/internal/pkg/generator"
	"github.com/yuzvak/flashsale-service/internal/pkg/logger"
)

type CheckoutCommand struct {
	UserID string
	ItemID string
}

type CheckoutResponse struct {
	Code       string    `json:"code"`
	ItemsCount int       `json:"items_count"`
	SaleEndsAt time.Time `json:"sale_ends_at"`
}

type CheckoutHandler struct {
	saleRepo      ports.SaleRepository
	checkoutRepo  ports.CheckoutRepository
	cache         ports.Cache
	log           *logger.Logger
	maxItemsLimit int
	codeGen       *generator.CodeGenerator
}

func NewCheckoutHandler(
	saleRepo ports.SaleRepository,
	checkoutRepo ports.CheckoutRepository,
	cache ports.Cache,
	log *logger.Logger,
	maxItemsLimit int,
	codeGen *generator.CodeGenerator,
) *CheckoutHandler {
	return &CheckoutHandler{
		saleRepo:      saleRepo,
		checkoutRepo:  checkoutRepo,
		cache:         cache,
		log:           log,
		maxItemsLimit: maxItemsLimit,
		codeGen:       codeGen,
	}
}

func (h *CheckoutHandler) Handle(ctx context.Context, cmd CheckoutCommand) (*CheckoutResponse, error) {
	activeSale, err := h.saleRepo.GetActiveSale(ctx)
	if err != nil {
		h.log.Error("Failed to get active sale", "error", err)
		return nil, errors.ErrSaleNotFound
	}

	if !activeSale.IsActive(time.Now().UTC()) {
		return nil, errors.ErrSaleNotActive
	}

	isSold, err := h.cache.ItemExistsInBloomFilter(ctx, cmd.ItemID)
	if err != nil {
		h.log.Error("Failed to check bloom filter", "error", err, "item_id", cmd.ItemID)
	} else if isSold {
		return nil, errors.ErrItemAlreadySold
	}

	availableSlots, err := h.cache.GetAvailableCheckoutSlots(ctx, activeSale.ID, cmd.UserID, h.maxItemsLimit)
	if err != nil {
		h.log.Error("Failed to get available checkout slots", "error", err, "user_id", cmd.UserID)
	} else if availableSlots <= 0 {
		return nil, errors.ErrUserLimitExceeded
	}

	hasCheckedOut, err := h.cache.HasUserCheckedOutItem(ctx, activeSale.ID, cmd.UserID, cmd.ItemID)
	if err != nil {
		h.log.Error("Failed to check user checkout history", "error", err, "user_id", cmd.UserID, "item_id", cmd.ItemID)
	} else if hasCheckedOut {
		return nil, errors.ErrUserAlreadyCheckedOutItem
	}

	item, err := h.saleRepo.GetItemByID(ctx, cmd.ItemID)
	if err != nil {
		h.log.Error("Failed to get item", "error", err, "item_id", cmd.ItemID)
		if err == errors.ErrItemNotFound {
			return nil, errors.ErrItemNotFound
		}
		return nil, err
	}

	if item.SaleID != activeSale.ID {
		return nil, errors.ErrItemNotInSale
	}

	if item.IsSold() {
		_ = h.cache.AddItemToBloomFilter(ctx, cmd.ItemID)
		return nil, errors.ErrItemAlreadySold
	}

	checkoutCode, err := h.cache.GetUserCheckoutCode(ctx, activeSale.ID, cmd.UserID)
	if err != nil || checkoutCode == "" {
		checkoutCode, err = h.codeGen.GenerateCheckoutCode(activeSale.ID, cmd.UserID)
		if err != nil {
			h.log.Error("Failed to generate checkout code", "error", err, "user_id", cmd.UserID)
			return nil, errors.ErrTransactionFailed
		}

		err = h.cache.SetUserCheckoutCode(ctx, activeSale.ID, cmd.UserID, checkoutCode, time.Until(activeSale.EndedAt))
		if err != nil {
			h.log.Error("Failed to set user checkout code", "error", err, "user_id", cmd.UserID)
		}

		err = h.cache.SetCheckoutCode(ctx, checkoutCode, time.Until(activeSale.EndedAt))
		if err != nil {
			h.log.Error("Failed to set checkout code", "error", err, "code", checkoutCode)
		}
	}

	checkout, err := h.checkoutRepo.GetCheckoutByCode(ctx, checkoutCode)
	if err != nil {
		checkout, err = sale.NewCheckout(checkoutCode, activeSale.ID, cmd.UserID, []string{cmd.ItemID})
		if err != nil {
			h.log.Error("Failed to create checkout", "error", err)
			return nil, err
		}

		err = h.checkoutRepo.CreateCheckout(ctx, checkout)
		if err != nil {
			h.log.Error("Failed to store checkout", "error", err)
			return nil, err
		}
	} else {
		err = checkout.AddItem(cmd.ItemID)
		if err != nil {
			return nil, errors.ErrUserAlreadyCheckedOutItem
		}

		err = h.checkoutRepo.AddItemToCheckout(ctx, checkoutCode, cmd.ItemID)
		if err != nil {
			h.log.Error("Failed to add item to checkout", "error", err)
			return nil, err
		}
	}

	err = h.cache.IncrementUserCheckoutCount(ctx, activeSale.ID, cmd.UserID)
	if err != nil {
		h.log.Error("Failed to increment user checkout count", "error", err, "user_id", cmd.UserID, "sale_id", activeSale.ID)
	}

	err = h.cache.AddUserCheckedOutItem(ctx, activeSale.ID, cmd.UserID, cmd.ItemID, time.Until(activeSale.EndedAt))
	if err != nil {
		h.log.Error("Failed to mark item as checked out by user", "error", err, "user_id", cmd.UserID, "item_id", cmd.ItemID, "sale_id", activeSale.ID)
	}

	return &CheckoutResponse{
		Code:       checkoutCode,
		ItemsCount: checkout.ItemCount(),
		SaleEndsAt: activeSale.EndedAt,
	}, nil
}
