package sale

import (
	"context"
)

type Repository interface {
	GetActiveSale(ctx context.Context) (*Sale, error)
	GetSaleByID(ctx context.Context, id string) (*Sale, error)
	CreateSale(ctx context.Context, sale *Sale) error
	UpdateSale(ctx context.Context, sale *Sale) error

	GetItemByID(ctx context.Context, id string) (*Item, error)
	GetItemsBySaleID(ctx context.Context, saleID string, limit, offset int) ([]*Item, error)
	GetAvailableItemsBySaleID(ctx context.Context, saleID string, limit, offset int) ([]*Item, error)
	CreateItem(ctx context.Context, item *Item) error
	CreateItems(ctx context.Context, items []*Item) error
	MarkItemAsSold(ctx context.Context, id string, userID string) (bool, error)

	GetCheckoutByCode(ctx context.Context, code string) (*Checkout, error)
	CreateCheckout(ctx context.Context, checkout *Checkout) error
	AddItemToCheckout(ctx context.Context, checkoutCode string, itemID string) error
	GetUserCheckoutCount(ctx context.Context, saleID, userID string) (int, error)

	BeginTx(ctx context.Context) (Repository, error)
	CommitTx(ctx context.Context) error
	RollbackTx(ctx context.Context) error
}
