package response

import (
	"errors"
	"net/http"

	domainErrors "github.com/yuzvak/flashsale-service/internal/domain/errors"
)

type ErrorMapping struct {
	HTTPStatus int
	Status     Status
	Message    string
}

var errorMappings = map[error]ErrorMapping{
	domainErrors.ErrSaleNotFound: {
		HTTPStatus: http.StatusNotFound,
		Status:     StatusNotFound,
		Message:    "Sale not found",
	},
	domainErrors.ErrSaleNotActive: {
		HTTPStatus: http.StatusBadRequest,
		Status:     StatusError,
		Message:    "Sale is not active",
	},
	domainErrors.ErrSaleOutOfStock: {
		HTTPStatus: http.StatusBadRequest,
		Status:     StatusError,
		Message:    "Sale is out of stock",
	},
	domainErrors.ErrSaleLimitExceeded: {
		HTTPStatus: http.StatusBadRequest,
		Status:     StatusError,
		Message:    "Purchase would exceed sale limit",
	},
	domainErrors.ErrNoItemsToPurchase: {
		HTTPStatus: http.StatusBadRequest,
		Status:     StatusError,
		Message:    "No items to purchase",
	},
	domainErrors.ErrItemNotFound: {
		HTTPStatus: http.StatusNotFound,
		Status:     StatusNotFound,
		Message:    "Item not found",
	},
	domainErrors.ErrItemAlreadySold: {
		HTTPStatus: http.StatusConflict,
		Status:     StatusConflict,
		Message:    "Items already sold",
	},
	domainErrors.ErrItemNotInSale: {
		HTTPStatus: http.StatusBadRequest,
		Status:     StatusError,
		Message:    "Item not in current sale",
	},
	domainErrors.ErrAllItemsSold: {
		HTTPStatus: http.StatusConflict,
		Status:     StatusConflict,
		Message:    "All items from checkout already sold",
	},
	domainErrors.ErrCheckoutNotFound: {
		HTTPStatus: http.StatusNotFound,
		Status:     StatusNotFound,
		Message:    "Checkout not found",
	},
	domainErrors.ErrCheckoutExpired: {
		HTTPStatus: http.StatusBadRequest,
		Status:     StatusError,
		Message:    "Checkout expired",
	},
	domainErrors.ErrItemAlreadyInCheckout: {
		HTTPStatus: http.StatusBadRequest,
		Status:     StatusError,
		Message:    "Item already in checkout",
	},
	domainErrors.ErrUserAlreadyCheckedOutItem: {
		HTTPStatus: http.StatusBadRequest,
		Status:     StatusError,
		Message:    "User already checked out this item",
	},
	domainErrors.ErrUserLimitExceeded: {
		HTTPStatus: http.StatusBadRequest,
		Status:     StatusError,
		Message:    "User has reached maximum items limit",
	},
	domainErrors.ErrCheckoutAlreadyProcessed: {
		HTTPStatus: http.StatusConflict,
		Status:     StatusConflict,
		Message:    "Checkout code has already been processed",
	},
	domainErrors.ErrTransactionFailed: {
		HTTPStatus: http.StatusInternalServerError,
		Status:     StatusInternalError,
		Message:    "Transaction failed",
	},
}

func MapDomainError(err error) (int, *ErrorResponse) {
	for domainErr, mapping := range errorMappings {
		if errors.Is(err, domainErr) {
			return mapping.HTTPStatus, Error(mapping.Status, mapping.Message, err.Error())
		}
	}

	return http.StatusInternalServerError, Error(StatusInternalError, "Internal server error", err.Error())
}

func WriteDomainError(w http.ResponseWriter, err error) {
	statusCode, errorResponse := MapDomainError(err)
	WriteJSON(w, statusCode, errorResponse)
}
