package ports

import (
	"context"

	"github.com/yuzvak/flashsale-service/internal/domain/sale"
)

type SaleRepository interface {
	GetActiveSale(ctx context.Context) (*sale.Sale, error)
	GetSaleByID(ctx context.Context, id string) (*sale.Sale, error)
	CreateSale(ctx context.Context, sale *sale.Sale) error
	UpdateSale(ctx context.Context, sale *sale.Sale) error

	GetItemByID(ctx context.Context, id string) (*sale.Item, error)
	GetItemsBySaleID(ctx context.Context, saleID string, limit, offset int) ([]*sale.Item, error)
	GetAvailableItemsBySaleID(ctx context.Context, saleID string, limit, offset int) ([]*sale.Item, error)
	CreateItem(ctx context.Context, item *sale.Item) error
	CreateItems(ctx context.Context, items []*sale.Item) error
	MarkItemAsSold(ctx context.Context, id string, userID string) (bool, error)

	SavePurchaseResult(ctx context.Context, checkoutCode string, result *sale.PurchaseResult) error
	GetPurchaseResult(ctx context.Context, checkoutCode string) (*sale.PurchaseResult, error)

	BeginTx(ctx context.Context) (SaleRepository, error)
	CommitTx(ctx context.Context) error
	RollbackTx(ctx context.Context) error
}
