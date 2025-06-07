package monitoring

import (
	"context"
)

type SaleMetrics struct {
	saleID string
}

func NewSaleMetrics(saleID string) *SaleMetrics {
	return &SaleMetrics{
		saleID: saleID,
	}
}

func (m *SaleMetrics) UpdateItemCounts(total, sold int) {
	UpdateSaleItemsCount(m.saleID, total, sold)
}

func (m *SaleMetrics) RecordItemSold(itemID string) {
	RecordItemSold(m.saleID, itemID)
}

type CheckoutMetrics struct {
	userID string
	itemID string
}

func NewCheckoutMetrics(userID, itemID string) *CheckoutMetrics {
	return &CheckoutMetrics{
		userID: userID,
		itemID: itemID,
	}
}

func (m *CheckoutMetrics) RecordAttempt() {
	RecordCheckoutAttempt(m.userID, m.itemID)
}

func (m *CheckoutMetrics) RecordSuccess() {
	RecordCheckoutSuccess(m.userID, m.itemID)
}

func (m *CheckoutMetrics) RecordFailure(reason string) {
	RecordCheckoutFailure(m.userID, m.itemID, reason)
}

type PurchaseMetrics struct {
	checkoutCode string
}

func NewPurchaseMetrics(checkoutCode string) *PurchaseMetrics {
	return &PurchaseMetrics{
		checkoutCode: checkoutCode,
	}
}

func (m *PurchaseMetrics) RecordAttempt() {
	RecordPurchaseAttempt(m.checkoutCode)
}

func (m *PurchaseMetrics) RecordSuccess() {
	RecordPurchaseSuccess(m.checkoutCode)
}

func (m *PurchaseMetrics) RecordFailure(reason string) {
	RecordPurchaseFailure(m.checkoutCode, reason)
}

type BusinessMetricsMiddleware struct{}

func NewBusinessMetricsMiddleware() *BusinessMetricsMiddleware {
	return &BusinessMetricsMiddleware{}
}

func (m *BusinessMetricsMiddleware) WrapCheckoutHandler(next func(ctx context.Context, userID, itemID string) (string, error)) func(ctx context.Context, userID, itemID string) (string, error) {
	return func(ctx context.Context, userID, itemID string) (string, error) {
		metrics := NewCheckoutMetrics(userID, itemID)
		metrics.RecordAttempt()

		checkoutCode, err := next(ctx, userID, itemID)
		if err != nil {
			metrics.RecordFailure(err.Error())
			return "", err
		}

		metrics.RecordSuccess()
		return checkoutCode, nil
	}
}

func (m *BusinessMetricsMiddleware) WrapPurchaseHandler(next func(ctx context.Context, checkoutCode string) (bool, error)) func(ctx context.Context, checkoutCode string) (bool, error) {
	return func(ctx context.Context, checkoutCode string) (bool, error) {
		metrics := NewPurchaseMetrics(checkoutCode)
		metrics.RecordAttempt()

		success, err := next(ctx, checkoutCode)
		if err != nil {
			metrics.RecordFailure(err.Error())
			return false, err
		}

		if !success {
			metrics.RecordFailure("unknown_failure")
			return false, nil
		}

		metrics.RecordSuccess()
		return true, nil
	}
}
