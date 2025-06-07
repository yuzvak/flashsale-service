package sale

import (
	"time"
)

type Item struct {
	ID           string
	SaleID       string
	Name         string
	ImageURL     string
	Sold         bool
	SoldToUserID string
	SoldAt       *time.Time
	CreatedAt    time.Time
}

func NewItem(id, saleID, name, imageURL string) *Item {
	return &Item{
		ID:        id,
		SaleID:    saleID,
		Name:      name,
		ImageURL:  imageURL,
		Sold:      false,
		CreatedAt: time.Now().UTC(),
	}
}

func (i *Item) MarkAsSold(userID string) {
	i.Sold = true
	i.SoldToUserID = userID
	now := time.Now().UTC()
	i.SoldAt = &now
}

func (i *Item) IsSold() bool {
	return i.Sold
}

func (i *Item) BelongsToSale(saleID string) bool {
	return i.SaleID == saleID
}
