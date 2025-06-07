package postgres

import (
	"context"
	"database/sql"

	"github.com/yuzvak/flashsale-service/internal/domain/errors"
	"github.com/yuzvak/flashsale-service/internal/domain/sale"
	"github.com/yuzvak/flashsale-service/internal/infrastructure/monitoring"
	"github.com/yuzvak/flashsale-service/internal/pkg/generator"
)

type CheckoutRepository struct {
	db            *sql.DB
	codeGenerator *generator.CodeGenerator
}

func NewCheckoutRepository(conn *Connection) *CheckoutRepository {
	return &CheckoutRepository{
		db:            conn.GetDB(),
		codeGenerator: generator.NewCodeGenerator(),
	}
}

func (r *CheckoutRepository) GetCheckoutByCode(ctx context.Context, code string) (*sale.Checkout, error) {
	checkoutQuery := `
		SELECT checkout_code, sale_id, user_id, created_at
		FROM checkout_attempts
		WHERE checkout_code = $1
		ORDER BY created_at DESC
		LIMIT 1
	`

	var checkout sale.Checkout
	row := monitoring.InstrumentQueryRow(ctx, r.db, "SELECT", "checkout_attempts", checkoutQuery, code)
	err := row.Scan(
		&checkout.Code, &checkout.SaleID, &checkout.UserID, &checkout.CreatedAt,
	)

	if err != nil {
		if err == sql.ErrNoRows {
			return nil, errors.ErrCheckoutNotFound
		}
		return nil, err
	}

	itemsQuery := `
		SELECT ci.item_id
		FROM checkout_items ci
		JOIN checkout_attempts ca ON ci.checkout_attempt_id = ca.id
		WHERE ca.checkout_code = $1
		ORDER BY ci.added_at
	`

	rows, err := monitoring.InstrumentQuery(ctx, r.db, "SELECT", "checkout_items", itemsQuery, code)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var itemIDs []string
	for rows.Next() {
		var itemID string
		if err := rows.Scan(&itemID); err != nil {
			return nil, err
		}
		itemIDs = append(itemIDs, itemID)
	}

	if err := rows.Err(); err != nil {
		return nil, err
	}

	checkout.ItemIDs = itemIDs
	return &checkout, nil
}

func (r *CheckoutRepository) CreateCheckout(ctx context.Context, checkout *sale.Checkout) error {
	id := r.codeGenerator.GenerateCheckoutID()

	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()

	query := `
		INSERT INTO checkout_attempts (id, checkout_code, sale_id, user_id, created_at)
		VALUES ($1, $2, $3, $4, $5)
	`

	_, err = tx.ExecContext(ctx, query,
		id, checkout.Code, checkout.SaleID, checkout.UserID, checkout.CreatedAt,
	)
	if err != nil {
		return err
	}

	for _, itemID := range checkout.ItemIDs {
		itemQuery := `
			INSERT INTO checkout_items (id, checkout_attempt_id, item_id, added_at)
			VALUES ($1, $2, $3, $4)
		`
		itemIDGen := r.codeGenerator.GenerateCheckoutID()
		_, err = tx.ExecContext(ctx, itemQuery,
			itemIDGen, id, itemID, checkout.CreatedAt,
		)
		if err != nil {
			return err
		}
	}

	return tx.Commit()
}

func (r *CheckoutRepository) AddItemToCheckout(ctx context.Context, checkoutCode string, itemID string) error {
	checkoutQuery := `
		SELECT id FROM checkout_attempts WHERE checkout_code = $1
	`
	var checkoutAttemptID string
	row := monitoring.InstrumentQueryRow(ctx, r.db, "SELECT", "checkout_attempts", checkoutQuery, checkoutCode)
	err := row.Scan(&checkoutAttemptID)
	if err != nil {
		if err == sql.ErrNoRows {
			return errors.ErrCheckoutNotFound
		}
		return err
	}

	checkQuery := `
		SELECT COUNT(*) FROM checkout_items 
		WHERE checkout_attempt_id = $1 AND item_id = $2
	`
	var count int
	row = monitoring.InstrumentQueryRow(ctx, r.db, "SELECT", "checkout_items", checkQuery, checkoutAttemptID, itemID)
	err = row.Scan(&count)
	if err != nil {
		return err
	}
	if count > 0 {
		return errors.ErrItemAlreadyInCheckout
	}

	insertQuery := `
		INSERT INTO checkout_items (id, checkout_attempt_id, item_id, added_at)
		VALUES ($1, $2, $3, NOW())
	`
	itemIDGen := r.codeGenerator.GenerateCheckoutID()
	_, err = monitoring.InstrumentExec(ctx, r.db, "INSERT", "checkout_items", insertQuery, itemIDGen, checkoutAttemptID, itemID)

	return err
}

func (r *CheckoutRepository) GetUserCheckoutCount(ctx context.Context, saleID, userID string) (int, error) {
	query := `
		SELECT COUNT(*)
		FROM checkout_attempts
		WHERE sale_id = $1 AND user_id = $2
	`

	var count int
	row := monitoring.InstrumentQueryRow(ctx, r.db, "SELECT", "checkout_attempts", query, saleID, userID)
	err := row.Scan(&count)
	if err != nil {
		return 0, err
	}

	return count, nil
}

func (r *CheckoutRepository) LogCheckoutAttempt(ctx context.Context, saleID, userID, checkoutCode string, itemID string) error {
	monitoring.RecordCheckoutAttempt(userID, itemID)
	id := r.codeGenerator.GenerateCheckoutID()

	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		monitoring.RecordCheckoutFailure(userID, itemID, "tx_begin_error")
		return err
	}
	defer tx.Rollback()

	query := `
		INSERT INTO checkout_attempts (id, checkout_code, sale_id, user_id, created_at)
		VALUES ($1, $2, $3, $4, NOW())
	`

	_, err = tx.ExecContext(ctx, query, id, checkoutCode, saleID, userID)
	if err != nil {
		monitoring.RecordCheckoutFailure(userID, itemID, "insert_attempt_error")
		return err
	}

	itemQuery := `
		INSERT INTO checkout_items (id, checkout_attempt_id, item_id, added_at)
		VALUES ($1, $2, $3, NOW())
	`
	itemIDGen := r.codeGenerator.GenerateCheckoutID()
	_, err = tx.ExecContext(ctx, itemQuery, itemIDGen, id, itemID)
	if err != nil {
		monitoring.RecordCheckoutFailure(userID, itemID, "insert_item_error")
		return err
	}

	err = tx.Commit()
	if err == nil {
		monitoring.RecordCheckoutSuccess(userID, itemID)
	} else {
		monitoring.RecordCheckoutFailure(userID, itemID, "commit_error")
	}
	return err
}

func (r *CheckoutRepository) DeleteCheckout(ctx context.Context, checkoutCode string) error {
	query := `DELETE FROM checkout_attempts WHERE checkout_code = $1`
	_, err := r.db.ExecContext(ctx, query, checkoutCode)
	return err
}
