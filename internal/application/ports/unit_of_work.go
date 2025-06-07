package ports

import "context"

type UnitOfWork interface {
	SaleRepository() SaleRepository
	CheckoutRepository() CheckoutRepository
	Begin(ctx context.Context) error
	Commit() error
	Rollback() error
}
