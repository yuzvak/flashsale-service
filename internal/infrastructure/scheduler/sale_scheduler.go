package scheduler

import (
	"context"
	"time"

	"github.com/yuzvak/flashsale-service/internal/domain/sale"
	"github.com/yuzvak/flashsale-service/internal/infrastructure/persistence/postgres"
	"github.com/yuzvak/flashsale-service/internal/pkg/generator"
	"github.com/yuzvak/flashsale-service/internal/pkg/logger"
)

type SaleScheduler struct {
	saleRepo      *postgres.SaleRepository
	itemGenerator *generator.ItemGenerator
	codeGenerator *generator.CodeGenerator
	logger        *logger.Logger
	totalItems    int
	stopChan      chan struct{}
}

func NewSaleScheduler(
	saleRepo *postgres.SaleRepository,
	logger *logger.Logger,
	totalItems int,
) *SaleScheduler {
	return &SaleScheduler{
		saleRepo:      saleRepo,
		itemGenerator: generator.NewItemGenerator(),
		codeGenerator: generator.NewCodeGenerator(),
		logger:        logger,
		totalItems:    totalItems,
		stopChan:      make(chan struct{}),
	}
}

func (s *SaleScheduler) Start(ctx context.Context) {
	s.logger.Info("Starting sale scheduler")
	
	if err := s.createSaleIfNeeded(ctx); err != nil {
		s.logger.Error("Failed to create initial sale", "error", err)
	}

	ticker := time.NewTicker(time.Hour)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			s.logger.Info("Sale scheduler stopped")
			return
		case <-s.stopChan:
			s.logger.Info("Sale scheduler stopped")
			return
		case <-ticker.C:
			if err := s.createSaleIfNeeded(ctx); err != nil {
				s.logger.Error("Failed to create scheduled sale", "error", err)
			}
		}
	}
}

func (s *SaleScheduler) Stop() {
	close(s.stopChan)
}

func (s *SaleScheduler) createSaleIfNeeded(ctx context.Context) error {
	activeSale, err := s.saleRepo.GetActiveSale(ctx)
	if err == nil && activeSale != nil {
		s.logger.Info("Active sale already exists", "sale_id", activeSale.ID)
		return nil
	}

	now := time.Now().UTC()
	startedAt := time.Date(now.Year(), now.Month(), now.Day(), now.Hour(), 0, 0, 0, time.UTC)
	endedAt := startedAt.Add(time.Hour)

	saleID := s.codeGenerator.GenerateSaleID()

	newSale := sale.Sale{
		ID:         saleID,
		StartedAt:  startedAt,
		EndedAt:    endedAt,
		TotalItems: s.totalItems,
		ItemsSold:  0,
		CreatedAt:  time.Now(),
	}

	err = s.saleRepo.CreateSale(ctx, &newSale)
	if err != nil {
		return err
	}

	items := make([]*sale.Item, 0, s.totalItems)
	for i := 0; i < s.totalItems; i++ {
		item := sale.NewItem(
			s.itemGenerator.GenerateItemID(),
			newSale.ID,
			s.itemGenerator.GenerateName(),
			s.itemGenerator.GenerateImageURL(),
		)
		items = append(items, item)
	}

	err = s.saleRepo.CreateItems(ctx, items)
	if err != nil {
		return err
	}

	s.logger.Info("Created new sale", "sale_id", saleID, "started_at", startedAt, "ended_at", endedAt, "total_items", s.totalItems)
	return nil
}