package errors

import (
	"errors"
)

var (
	ErrSaleNotFound      = errors.New("sale not found")
	ErrSaleNotActive     = errors.New("sale is not active")
	ErrSaleOutOfStock    = errors.New("sale is out of stock")
	ErrSaleLimitExceeded = errors.New("purchase would exceed sale limit")
	ErrNoItemsToPurchase = errors.New("no items to purchase")

	ErrItemNotFound    = errors.New("item not found")
	ErrItemAlreadySold = errors.New("item already sold")
	ErrItemNotInSale   = errors.New("item not in current sale")
	ErrAllItemsSold    = errors.New("all items from checkout already sold")

	ErrCheckoutNotFound          = errors.New("checkout not found")
	ErrCheckoutExpired           = errors.New("checkout expired")
	ErrItemAlreadyInCheckout     = errors.New("item already in checkout")
	ErrUserAlreadyCheckedOutItem = errors.New("user already checked out this item")

	ErrUserLimitExceeded = errors.New("user has reached maximum items limit")

	ErrCheckoutAlreadyProcessed = errors.New("checkout code has already been processed")

	ErrTransactionFailed = errors.New("transaction failed")
)
