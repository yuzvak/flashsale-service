package commands

import (
	"context"

	"github.com/yuzvak/flashsale-service/internal/application/use_cases"
	"github.com/yuzvak/flashsale-service/internal/domain/sale"
	"github.com/yuzvak/flashsale-service/internal/pkg/logger"
)

type PurchaseCommand struct {
	CheckoutCode string
}

type PurchaseResponse struct {
	Success        bool                      `json:"success"`
	PurchasedItems []sale.PurchaseItemResult `json:"purchased_items"`
	TotalPurchased int                       `json:"total_purchased"`
	FailedCount    int                       `json:"failed_count"`
}

type PurchaseHandler struct {
	purchaseUseCase *use_cases.PurchaseUseCase
	log             *logger.Logger
}

func NewPurchaseHandler(
	purchaseUseCase *use_cases.PurchaseUseCase,
	log *logger.Logger,
) *PurchaseHandler {
	return &PurchaseHandler{
		purchaseUseCase: purchaseUseCase,
		log:             log,
	}
}

func (h *PurchaseHandler) Handle(ctx context.Context, cmd PurchaseCommand) (*PurchaseResponse, error) {
	h.log.Info("Processing purchase request", "checkout_code", cmd.CheckoutCode)

	result, err := h.purchaseUseCase.ExecutePurchase(ctx, cmd.CheckoutCode)
	if err != nil {
		h.log.Error("Purchase failed", "error", err.Error(), "checkout_code", cmd.CheckoutCode)
		return nil, err
	}

	response := &PurchaseResponse{
		Success:        result.Success,
		PurchasedItems: result.Items,
		TotalPurchased: result.TotalPurchased,
		FailedCount:    result.FailedCount,
	}

	h.log.Info("Purchase completed successfully",
		"checkout_code", cmd.CheckoutCode,
		"total_purchased", result.TotalPurchased,
		"failed_count", result.FailedCount,
	)

	return response, nil
}
