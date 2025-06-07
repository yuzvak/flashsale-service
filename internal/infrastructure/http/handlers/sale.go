package handlers

import (
	"net/http"
	"strings"
	"time"

	"github.com/yuzvak/flashsale-service/internal/domain/errors"
	"github.com/yuzvak/flashsale-service/internal/infrastructure/http/response"
	"github.com/yuzvak/flashsale-service/internal/infrastructure/persistence/postgres"
	"github.com/yuzvak/flashsale-service/internal/pkg/logger"
)

type SaleHandler struct {
	saleRepo *postgres.SaleRepository
	logger   *logger.Logger
}

func NewSaleHandler(saleRepo *postgres.SaleRepository, logger *logger.Logger) *SaleHandler {
	return &SaleHandler{
		saleRepo: saleRepo,
		logger:   logger,
	}
}

type SaleResponse struct {
	ID         string `json:"id"`
	StartedAt  string `json:"started_at"`
	EndedAt    string `json:"ended_at"`
	TotalItems int    `json:"total_items"`
	ItemsSold  int    `json:"items_sold"`
	Active     bool   `json:"active"`
}

type ItemResponse struct {
	ID       string `json:"id"`
	Name     string `json:"name"`
	ImageURL string `json:"image_url"`
	Sold     bool   `json:"sold"`
}

func (h *SaleHandler) HandleGetActiveSale(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()

	sale, err := h.saleRepo.GetActiveSale(ctx)
	if err != nil {
		if err == errors.ErrSaleNotFound {
			response.WriteDomainError(w, err)
			return
		}

		h.logger.Error("Failed to get active sale", map[string]interface{}{"error": err.Error()})
		response.WriteDomainError(w, err)
		return
	}

	saleResponse := SaleResponse{
		ID:         sale.ID,
		StartedAt:  sale.StartedAt.Format(time.RFC3339),
		EndedAt:    sale.EndedAt.Format(time.RFC3339),
		TotalItems: sale.TotalItems,
		ItemsSold:  sale.ItemsSold,
		Active:     true,
	}

	response.WriteSuccess(w, saleResponse)
}

func (h *SaleHandler) HandleGetSale(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	path := strings.TrimPrefix(r.URL.Path, "/api/v1/sales/")
	parts := strings.Split(path, "/")
	saleID := parts[0]

	if saleID == "" {
		response.WriteValidationError(w, "Validation failed", map[string]string{
			"sale_id": "Sale ID is required",
		})
		return
	}

	sale, err := h.saleRepo.GetSaleByID(ctx, saleID)
	if err != nil {
		if err == errors.ErrSaleNotFound {
			response.WriteDomainError(w, err)
			return
		}

		h.logger.Error("Failed to get sale", map[string]interface{}{"error": err.Error(), "sale_id": saleID})
		response.WriteDomainError(w, err)
		return
	}

	active := sale.StartedAt.Before(time.Now().UTC()) && sale.EndedAt.After(time.Now().UTC())

	saleResponse := SaleResponse{
		ID:         sale.ID,
		StartedAt:  sale.StartedAt.Format(time.RFC3339),
		EndedAt:    sale.EndedAt.Format(time.RFC3339),
		TotalItems: sale.TotalItems,
		ItemsSold:  sale.ItemsSold,
		Active:     active,
	}

	response.WriteSuccess(w, saleResponse)
}

func (h *SaleHandler) HandleGetSaleItems(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	path := strings.TrimPrefix(r.URL.Path, "/api/v1/sales/")
	parts := strings.Split(path, "/")
	saleID := parts[0]

	if saleID == "" {
		response.WriteValidationError(w, "Validation failed", map[string]string{
			"sale_id": "Sale ID is required",
		})
		return
	}

	_, err := h.saleRepo.GetSaleByID(ctx, saleID)
	if err != nil {
		if err == errors.ErrSaleNotFound {
			response.WriteDomainError(w, err)
			return
		}

		h.logger.Error("Failed to get sale", map[string]interface{}{"error": err.Error(), "sale_id": saleID})
		response.WriteDomainError(w, err)
		return
	}

	items, err := h.saleRepo.GetItemsBySaleID(ctx, saleID, 0, 100) // Default pagination: page 0, limit 100
	if err != nil {
		h.logger.Error("Failed to get items", map[string]interface{}{"error": err.Error(), "sale_id": saleID})
		response.WriteDomainError(w, err)
		return
	}

	responses := make([]ItemResponse, 0, len(items))
	for _, item := range items {
		responses = append(responses, ItemResponse{
			ID:       item.ID,
			Name:     item.Name,
			ImageURL: item.ImageURL,
			Sold:     item.Sold,
		})
	}

	response.WriteSuccess(w, responses)
}
