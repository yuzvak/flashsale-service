package postgres

import (
	"context"
	"database/sql"
	"time"

	"github.com/yuzvak/flashsale-service/internal/infrastructure/monitoring"
)

type Purchase struct {
	ID           string
	SaleID       string
	UserID       string
	ItemID       string
	CheckoutCode string
	PurchasedAt  time.Time
	CreatedAt    time.Time
}

type PurchaseRepository struct {
	conn *Connection
}

func NewPurchaseRepository(conn *Connection) *PurchaseRepository {
	return &PurchaseRepository{
		conn: conn,
	}
}

func (r *PurchaseRepository) CreatePurchase(ctx context.Context, tx *sql.Tx, saleID, userID, itemID, checkoutCode string) error {
	query := `
		INSERT INTO purchases (sale_id, user_id, item_id, checkout_code, purchased_at)
		VALUES ($1, $2, $3, $4, NOW())
	`

	_, err := tx.ExecContext(ctx, query, saleID, userID, itemID, checkoutCode)
	if err == nil {
		monitoring.PurchaseSuccessTotal.Inc()
	} else {
		monitoring.PurchaseFailureTotal.WithLabelValues("insert_error").Inc()
	}
	return err
}

func (r *PurchaseRepository) GetUserPurchaseCount(ctx context.Context, saleID, userID string) (int, error) {
	query := `
		SELECT COUNT(*)
		FROM purchases
		WHERE sale_id = $1 AND user_id = $2
	`

	var count int
	row := monitoring.InstrumentQueryRow(ctx, r.conn.db, "SELECT", "purchases", query, saleID, userID)
	err := row.Scan(&count)
	return count, err
}

func (r *PurchaseRepository) GetPurchasesBySaleID(ctx context.Context, saleID string) ([]Purchase, error) {
	query := `
		SELECT id, sale_id, user_id, item_id, checkout_code, purchased_at
		FROM purchases
		WHERE sale_id = $1
		ORDER BY purchased_at
	`

	rows, err := monitoring.InstrumentQuery(ctx, r.conn.db, "SELECT", "purchases", query, saleID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var purchases []Purchase
	for rows.Next() {
		var p Purchase
		if err := rows.Scan(&p.ID, &p.SaleID, &p.UserID, &p.ItemID, &p.CheckoutCode, &p.PurchasedAt); err != nil {
			return nil, err
		}
		purchases = append(purchases, p)
	}

	return purchases, rows.Err()
}

func (r *PurchaseRepository) GetPurchasesByUserID(ctx context.Context, userID string) ([]Purchase, error) {
	query := `
		SELECT id, sale_id, user_id, item_id, checkout_code, purchased_at
		FROM purchases
		WHERE user_id = $1
		ORDER BY purchased_at DESC
	`

	rows, err := monitoring.InstrumentQuery(ctx, r.conn.db, "SELECT", "purchases", query, userID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var purchases []Purchase
	for rows.Next() {
		var p Purchase
		if err := rows.Scan(&p.ID, &p.SaleID, &p.UserID, &p.ItemID, &p.CheckoutCode, &p.PurchasedAt); err != nil {
			return nil, err
		}
		purchases = append(purchases, p)
	}

	return purchases, rows.Err()
}
