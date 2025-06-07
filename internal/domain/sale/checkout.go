package sale

import (
	"errors"
	"fmt"
	"time"
)

type Checkout struct {
	Code      string
	SaleID    string
	UserID    string
	ItemIDs   []string
	CreatedAt time.Time
}

func NewCheckout(code, saleID, userID string, itemIDs []string) (*Checkout, error) {
	if code == "" {
		return nil, errors.New("checkout code cannot be empty")
	}

	if saleID == "" {
		return nil, errors.New("sale id cannot be empty")
	}

	if userID == "" {
		return nil, errors.New("user id cannot be empty")
	}

	if len(itemIDs) == 0 {
		return nil, errors.New("item ids cannot be empty")
	}

	return &Checkout{
		Code:      code,
		SaleID:    saleID,
		UserID:    userID,
		ItemIDs:   itemIDs,
		CreatedAt: time.Now().UTC(),
	}, nil
}

func (c *Checkout) AddItem(itemID string) error {
	for _, id := range c.ItemIDs {
		if id == itemID {
			return errors.New("item already in checkout")
		}
	}

	c.ItemIDs = append(c.ItemIDs, itemID)
	return nil
}

func (c *Checkout) ItemCount() int {
	return len(c.ItemIDs)
}

func GenerateCode(saleID, userID string) string {
	return fmt.Sprintf("CHK-%s-%s", saleID, "random")
}
