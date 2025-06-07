package user

import (
	"errors"
)

type Limits struct {
	UserID           string
	SaleID           string
	MaxItemsPerUser  int
	CurrentItemCount int
}

func NewLimits(userID, saleID string, maxItemsPerUser int) *Limits {
	return &Limits{
		UserID:           userID,
		SaleID:           saleID,
		MaxItemsPerUser:  maxItemsPerUser,
		CurrentItemCount: 0,
	}
}

func (l *Limits) CanAddItems(count int) bool {
	return l.CurrentItemCount+count <= l.MaxItemsPerUser
}

func (l *Limits) IncrementItemCount() {
	l.CurrentItemCount++
}

func (l *Limits) AddItems(count int) error {
	if !l.CanAddItems(count) {
		return errors.New("user has reached maximum items limit")
	}

	l.CurrentItemCount += count
	return nil
}
