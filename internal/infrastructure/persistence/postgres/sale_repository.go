package postgres

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"

	"github.com/yuzvak/flashsale-service/internal/application/ports"
	domainErrors "github.com/yuzvak/flashsale-service/internal/domain/errors"
	"github.com/yuzvak/flashsale-service/internal/domain/sale"
	"github.com/yuzvak/flashsale-service/internal/infrastructure/monitoring"
)

type SaleRepository struct {
	db   *sql.DB
	tx   *sql.Tx
	isTx bool
}

func NewSaleRepository(conn *Connection) *SaleRepository {
	return &SaleRepository{
		db:   conn.GetDB(),
		isTx: false,
	}
}

func (r *SaleRepository) GetActiveSale(ctx context.Context) (*sale.Sale, error) {
	query := `
		SELECT id, started_at, ended_at, total_items, items_sold, created_at
		FROM sales
		WHERE started_at <= NOW() AND ended_at > NOW()
		ORDER BY started_at DESC
		LIMIT 1
	`

	var s sale.Sale
	var err error

	if r.isTx {
		err = r.tx.QueryRowContext(ctx, query).Scan(
			&s.ID, &s.StartedAt, &s.EndedAt, &s.TotalItems, &s.ItemsSold, &s.CreatedAt,
		)
	} else {
		row := monitoring.InstrumentQueryRow(ctx, r.db, "SELECT", "sales", query)
		err = row.Scan(&s.ID, &s.StartedAt, &s.EndedAt, &s.TotalItems, &s.ItemsSold, &s.CreatedAt)
	}

	if err != nil {
		if err == sql.ErrNoRows {
			return nil, domainErrors.ErrSaleNotFound
		}
		return nil, err
	}

	monitoring.UpdateSaleItemsCount(s.ID, s.TotalItems, s.ItemsSold)

	return &s, nil
}

func (r *SaleRepository) GetSaleByID(ctx context.Context, id string) (*sale.Sale, error) {
	query := `
		SELECT id, started_at, ended_at, total_items, items_sold, created_at
		FROM sales
		WHERE id = $1
	`

	var s sale.Sale
	var err error

	if r.isTx {
		err = r.tx.QueryRowContext(ctx, query, id).Scan(
			&s.ID, &s.StartedAt, &s.EndedAt, &s.TotalItems, &s.ItemsSold, &s.CreatedAt,
		)
	} else {
		row := monitoring.InstrumentQueryRow(ctx, r.db, "SELECT", "sales", query, id)
		err = row.Scan(&s.ID, &s.StartedAt, &s.EndedAt, &s.TotalItems, &s.ItemsSold, &s.CreatedAt)
	}

	if err != nil {
		if err == sql.ErrNoRows {
			return nil, domainErrors.ErrSaleNotFound
		}
		return nil, err
	}

	monitoring.UpdateSaleItemsCount(s.ID, s.TotalItems, s.ItemsSold)

	return &s, nil
}

func (r *SaleRepository) CreateSale(ctx context.Context, s *sale.Sale) error {
	query := `
		INSERT INTO sales (id, started_at, ended_at, total_items, items_sold, created_at)
		VALUES ($1, $2, $3, $4, $5, $6)
	`

	var err error

	if r.isTx {
		_, err = r.tx.ExecContext(ctx, query,
			s.ID, s.StartedAt, s.EndedAt, s.TotalItems, s.ItemsSold, s.CreatedAt,
		)
	} else {
		_, err = monitoring.InstrumentExec(ctx, r.db, "INSERT", "sales", query,
			s.ID, s.StartedAt, s.EndedAt, s.TotalItems, s.ItemsSold, s.CreatedAt,
		)
	}

	return err
}

func (r *SaleRepository) UpdateSale(ctx context.Context, s *sale.Sale) error {
	query := `
		UPDATE sales
		SET started_at = $2, ended_at = $3, total_items = $4, items_sold = $5
		WHERE id = $1
	`

	var err error

	if r.isTx {
		_, err = r.tx.ExecContext(ctx, query,
			s.ID, s.StartedAt, s.EndedAt, s.TotalItems, s.ItemsSold,
		)
	} else {
		_, err = monitoring.InstrumentExec(ctx, r.db, "UPDATE", "sales", query,
			s.ID, s.StartedAt, s.EndedAt, s.TotalItems, s.ItemsSold,
		)
	}

	if err == nil {
		monitoring.UpdateSaleItemsCount(s.ID, s.TotalItems, s.ItemsSold)
	}

	return err
}

func (r *SaleRepository) GetItemByID(ctx context.Context, id string) (*sale.Item, error) {
	query := `
		SELECT id, sale_id, name, image_url, sold, sold_to_user_id, sold_at, created_at
		FROM items
		WHERE id = $1
	`

	var item sale.Item
	var soldToUserID sql.NullString
	var soldAt sql.NullTime
	var err error

	if r.isTx {
		err = r.tx.QueryRowContext(ctx, query, id).Scan(
			&item.ID, &item.SaleID, &item.Name, &item.ImageURL, &item.Sold,
			&soldToUserID, &soldAt, &item.CreatedAt,
		)
	} else {
		row := monitoring.InstrumentQueryRow(ctx, r.db, "SELECT", "items", query, id)
		err = row.Scan(&item.ID, &item.SaleID, &item.Name, &item.ImageURL, &item.Sold,
			&soldToUserID, &soldAt, &item.CreatedAt,
		)
	}

	if err != nil {
		if err == sql.ErrNoRows {
			return nil, domainErrors.ErrItemNotFound
		}
		return nil, err
	}

	if soldToUserID.Valid {
		item.SoldToUserID = soldToUserID.String
	}

	if soldAt.Valid {
		item.SoldAt = &soldAt.Time
	}

	return &item, nil
}

func (r *SaleRepository) GetItemsBySaleID(ctx context.Context, saleID string, limit, offset int) ([]*sale.Item, error) {
	query := `
		SELECT id, sale_id, name, image_url, sold, sold_to_user_id, sold_at, created_at
		FROM items
		WHERE sale_id = $1
		ORDER BY created_at
		LIMIT $2 OFFSET $3
	`

	var rows *sql.Rows
	var err error

	if r.isTx {
		rows, err = r.tx.QueryContext(ctx, query, saleID, limit, offset)
	} else {
		rows, err = monitoring.InstrumentQuery(ctx, r.db, "SELECT", "items", query, saleID, limit, offset)
	}

	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var items []*sale.Item

	for rows.Next() {
		var item sale.Item
		var soldToUserID sql.NullString
		var soldAt sql.NullTime

		err := rows.Scan(
			&item.ID, &item.SaleID, &item.Name, &item.ImageURL, &item.Sold,
			&soldToUserID, &soldAt, &item.CreatedAt,
		)
		if err != nil {
			return nil, err
		}

		if soldToUserID.Valid {
			item.SoldToUserID = soldToUserID.String
		}

		if soldAt.Valid {
			item.SoldAt = &soldAt.Time
		}

		if item.Sold {
			monitoring.SaleItemsSold.Add(1)
		} else {
			monitoring.SaleItemsTotal.Add(1)
		}

		items = append(items, &item)
	}

	return items, nil
}

func (r *SaleRepository) GetAvailableItemsBySaleID(ctx context.Context, saleID string, limit, offset int) ([]*sale.Item, error) {
	query := `
		SELECT id, sale_id, name, image_url, sold, sold_to_user_id, sold_at, created_at
		FROM items
		WHERE sale_id = $1 AND sold = FALSE
		ORDER BY created_at
		LIMIT $2 OFFSET $3
	`

	var rows *sql.Rows
	var err error

	if r.isTx {
		rows, err = r.tx.QueryContext(ctx, query, saleID, limit, offset)
	} else {
		rows, err = monitoring.InstrumentQuery(ctx, r.db, "SELECT", "items", query, saleID, limit, offset)
	}

	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var items []*sale.Item

	for rows.Next() {
		var item sale.Item
		var soldToUserID sql.NullString
		var soldAt sql.NullTime

		err := rows.Scan(
			&item.ID, &item.SaleID, &item.Name, &item.ImageURL, &item.Sold,
			&soldToUserID, &soldAt, &item.CreatedAt,
		)
		if err != nil {
			return nil, err
		}

		if soldToUserID.Valid {
			item.SoldToUserID = soldToUserID.String
		}

		if soldAt.Valid {
			item.SoldAt = &soldAt.Time
		}

		if item.Sold {
			monitoring.SaleItemsSold.Add(1)
		} else {
			monitoring.SaleItemsTotal.Add(1)
		}

		items = append(items, &item)
	}

	return items, nil
}

func (r *SaleRepository) CreateItem(ctx context.Context, item *sale.Item) error {
	query := `
		INSERT INTO items (id, sale_id, name, image_url, sold, created_at)
		VALUES ($1, $2, $3, $4, $5, $6)
	`

	var err error

	if r.isTx {
		_, err = r.tx.ExecContext(ctx, query,
			item.ID, item.SaleID, item.Name, item.ImageURL, item.Sold, item.CreatedAt,
		)
	} else {
		_, err = monitoring.InstrumentExec(ctx, r.db, "INSERT", "items", query,
			item.ID, item.SaleID, item.Name, item.ImageURL, item.Sold, item.CreatedAt,
		)
	}

	return err
}

func (r *SaleRepository) CreateItems(ctx context.Context, items []*sale.Item) error {
	if len(items) == 0 {
		return nil
	}

	var tx *sql.Tx
	var err error

	if r.isTx {
		tx = r.tx
	} else {
		tx, err = r.db.BeginTx(ctx, nil)
		if err != nil {
			return err
		}
		defer func() {
			if err != nil {
				tx.Rollback()
			}
		}()
	}

	stmt, err := tx.PrepareContext(ctx, `
		INSERT INTO items (id, sale_id, name, image_url, sold, created_at)
		VALUES ($1, $2, $3, $4, $5, $6)
	`)
	if err != nil {
		return err
	}
	defer stmt.Close()

	for _, item := range items {
		_, err = stmt.ExecContext(ctx,
			item.ID, item.SaleID, item.Name, item.ImageURL, item.Sold, item.CreatedAt,
		)
		if err != nil {
			return err
		}
	}

	if !r.isTx {
		return tx.Commit()
	}

	return nil
}

func (r *SaleRepository) MarkItemAsSold(ctx context.Context, id string, userID string) (bool, error) {
	query := `
		UPDATE items
		SET sold = TRUE, sold_to_user_id = $2, sold_at = NOW()
		WHERE id = $1 AND sold = FALSE
	`

	var result sql.Result
	var err error

	if r.isTx {
		result, err = r.tx.ExecContext(ctx, query, id, userID)
	} else {
		result, err = monitoring.InstrumentExec(ctx, r.db, "UPDATE", "items", query, id, userID)
	}

	if err != nil {
		return false, err
	}

	rowsAffected, err := result.RowsAffected()
	if err != nil {
		return false, err
	}

	success := rowsAffected > 0
	if success {
		var saleID string
		getSaleQuery := "SELECT sale_id FROM items WHERE id = $1"
		if r.isTx {
			err = r.tx.QueryRowContext(ctx, getSaleQuery, id).Scan(&saleID)
		} else {
			err = r.db.QueryRowContext(ctx, getSaleQuery, id).Scan(&saleID)
		}
		if err == nil {
			monitoring.RecordItemSold(saleID, id)
		}
	}

	return success, nil
}

func (r *SaleRepository) BeginTx(ctx context.Context) (ports.SaleRepository, error) {
	if r.isTx {
		return nil, errors.New("transaction already started")
	}

	tx, err := r.db.BeginTx(ctx, &sql.TxOptions{
		Isolation: sql.LevelSerializable, // Highest isolation level for critical operations
	})
	if err != nil {
		return nil, err
	}

	return &SaleRepository{
		db:   r.db,
		tx:   tx,
		isTx: true,
	}, nil
}

func (r *SaleRepository) CommitTx(ctx context.Context) error {
	if !r.isTx || r.tx == nil {
		return errors.New("no transaction to commit")
	}

	return r.tx.Commit()
}

func (r *SaleRepository) RollbackTx(ctx context.Context) error {
	if !r.isTx || r.tx == nil {
		return errors.New("no transaction to rollback")
	}

	return r.tx.Rollback()
}

func (r *SaleRepository) SavePurchaseResult(ctx context.Context, checkoutCode string, result *sale.PurchaseResult) error {
	query := `
		INSERT INTO purchase_results (checkout_code, result, created_at)
		VALUES ($1, $2, NOW())
		ON CONFLICT (checkout_code) DO NOTHING
	`

	resultJSON, err := json.Marshal(result)
	if err != nil {
		return err
	}

	if r.isTx {
		_, err = r.tx.ExecContext(ctx, query, checkoutCode, resultJSON)
	} else {
		_, err = monitoring.InstrumentExec(ctx, r.db, "INSERT", "purchase_results", query, checkoutCode, resultJSON)
	}

	return err
}

func (r *SaleRepository) GetPurchaseResult(ctx context.Context, checkoutCode string) (*sale.PurchaseResult, error) {
	query := `
		SELECT result FROM purchase_results
		WHERE checkout_code = $1
	`

	var resultJSON []byte
	var err error

	if r.isTx {
		err = r.tx.QueryRowContext(ctx, query, checkoutCode).Scan(&resultJSON)
	} else {
		row := monitoring.InstrumentQueryRow(ctx, r.db, "SELECT", "purchase_results", query, checkoutCode)
		err = row.Scan(&resultJSON)
	}

	if err != nil {
		if err == sql.ErrNoRows {
			return nil, nil
		}
		return nil, err
	}

	var result sale.PurchaseResult
	err = json.Unmarshal(resultJSON, &result)
	if err != nil {
		return nil, err
	}

	return &result, nil
}
