package ports

import (
	"context"

	"github.com/yuzvak/flashsale-service/internal/domain/sale"
)

type CheckoutRepository interface {
	GetCheckoutByCode(ctx context.Context, code string) (*sale.Checkout, error)
	CreateCheckout(ctx context.Context, checkout *sale.Checkout) error
	AddItemToCheckout(ctx context.Context, checkoutCode string, itemID string) error
	GetUserCheckoutCount(ctx context.Context, saleID, userID string) (int, error)
	DeleteCheckout(ctx context.Context, checkoutCode string) error

	LogCheckoutAttempt(ctx context.Context, saleID, userID, checkoutCode string, itemID string) error
}
