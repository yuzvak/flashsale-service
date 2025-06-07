package sale

import (
	"errors"
	"time"
)

type Sale struct {
	ID         string // Format: YYYYMMDDHH
	StartedAt  time.Time
	EndedAt    time.Time
	TotalItems int
	ItemsSold  int
	CreatedAt  time.Time
}

func NewSale(id string, startedAt, endedAt time.Time, totalItems int) (*Sale, error) {
	if id == "" {
		return nil, errors.New("sale id cannot be empty")
	}

	if startedAt.After(endedAt) || startedAt.Equal(endedAt) {
		return nil, errors.New("start time must be before end time")
	}

	if totalItems <= 0 {
		return nil, errors.New("total items must be greater than zero")
	}

	return &Sale{
		ID:         id,
		StartedAt:  startedAt,
		EndedAt:    endedAt,
		TotalItems: totalItems,
		ItemsSold:  0,
		CreatedAt:  time.Now().UTC(),
	}, nil
}

func (s *Sale) IsActive(now time.Time) bool {
	return now.After(s.StartedAt) && now.Before(s.EndedAt)
}

func (s *Sale) HasAvailableItems() bool {
	return s.ItemsSold < s.TotalItems
}

func (s *Sale) IncrementItemsSold() {
	s.ItemsSold++
}
