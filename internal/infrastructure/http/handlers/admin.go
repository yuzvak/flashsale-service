package handlers

import (
	"encoding/json"
	"errors"
	"net/http"
	"time"

	domainErrors "github.com/yuzvak/flashsale-service/internal/domain/errors"
	"github.com/yuzvak/flashsale-service/internal/domain/sale"
	"github.com/yuzvak/flashsale-service/internal/infrastructure/http/response"
	"github.com/yuzvak/flashsale-service/internal/infrastructure/persistence/postgres"
	"github.com/yuzvak/flashsale-service/internal/pkg/generator"
	"github.com/yuzvak/flashsale-service/internal/pkg/logger"
)

type AdminHandler struct {
	saleRepo      *postgres.SaleRepository
	itemGenerator *generator.ItemGenerator
	codeGenerator *generator.CodeGenerator
	logger        *logger.Logger
}

func NewAdminHandler(
	saleRepo *postgres.SaleRepository,
	logger *logger.Logger,
) *AdminHandler {
	return &AdminHandler{
		saleRepo:      saleRepo,
		itemGenerator: generator.NewItemGenerator(),
		codeGenerator: generator.NewCodeGenerator(),
		logger:        logger,
	}
}

type CreateSaleRequest struct {
	StartedAt  string `json:"started_at,omitempty"`
	EndedAt    string `json:"ended_at,omitempty"`
	TotalItems int    `json:"total_items"`
}

type CreateSaleResponse struct {
	ID         string `json:"id"`
	StartedAt  string `json:"started_at"`
	EndedAt    string `json:"ended_at"`
	TotalItems int    `json:"total_items"`
}

func (h *AdminHandler) HandleCreateSale(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()

	var req CreateSaleRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		response.WriteError(w, http.StatusBadRequest, response.StatusValidationError, "Invalid request body", err.Error())
		return
	}

	validationErrors := make(map[string]string)
	if req.TotalItems <= 0 {
		validationErrors["total_items"] = "Total items must be greater than 0"
	}

	var startedAt, endedAt time.Time
	var err error

	if req.StartedAt != "" {
		startedAt, err = time.Parse(time.RFC3339, req.StartedAt)
		if err != nil {
			validationErrors["started_at"] = "Invalid started_at time format (use RFC3339)"
		}
	} else {
		now := time.Now().UTC()
		startedAt = time.Date(now.Year(), now.Month(), now.Day(), now.Hour(), 0, 0, 0, time.UTC)
	}

	if req.EndedAt != "" {
		endedAt, err = time.Parse(time.RFC3339, req.EndedAt)
		if err != nil {
			validationErrors["ended_at"] = "Invalid ended_at time format (use RFC3339)"
		}
	} else {
		if !startedAt.IsZero() {
			endedAt = startedAt.Add(time.Hour)
		}
	}

	if !startedAt.IsZero() && !endedAt.IsZero() && startedAt.After(endedAt) {
		validationErrors["started_at"] = "started_at must be before ended_at"
	}

	if len(validationErrors) > 0 {
		response.WriteValidationError(w, "Validation failed", validationErrors)
		return
	}

	saleID := h.codeGenerator.GenerateSaleID()

	newSale := sale.Sale{
		ID:         saleID,
		StartedAt:  startedAt,
		EndedAt:    endedAt,
		TotalItems: req.TotalItems,
		ItemsSold:  0,
		CreatedAt:  time.Now(),
	}

	activeSale, err := h.saleRepo.GetActiveSale(ctx)
	if err != nil && !errors.Is(err, domainErrors.ErrSaleNotFound) {
		h.logger.Error("Failed to check active sales", map[string]interface{}{"error": err.Error()})
		response.WriteError(w, http.StatusInternalServerError, response.StatusInternalError, "Failed to check active sales", err.Error())
		return
	}

	if activeSale != nil {
		response.WriteError(w, http.StatusConflict, response.StatusValidationError, "Cannot create new sale", "A sale is currently active. Wait until it ends before creating a new one.")
		return
	}

	err = h.saleRepo.CreateSale(ctx, &newSale)
	if err != nil {
		h.logger.Error("Failed to create sale", map[string]interface{}{"error": err.Error()})
		response.WriteError(w, http.StatusInternalServerError, response.StatusInternalError, "Failed to create sale", err.Error())
		return
	}

	items := make([]*sale.Item, 0, req.TotalItems)
	for i := 0; i < req.TotalItems; i++ {
		item := sale.NewItem(h.itemGenerator.GenerateItemID(), newSale.ID, h.itemGenerator.GenerateName(), h.itemGenerator.GenerateImageURL())
		items = append(items, item)
	}

	err = h.saleRepo.CreateItems(ctx, items)
	if err != nil {
		h.logger.Error("Failed to create items", map[string]interface{}{"error": err.Error(), "sale_id": saleID})
		response.WriteError(w, http.StatusInternalServerError, response.StatusInternalError, "Failed to create items", err.Error())
		return
	}

	saleResponse := CreateSaleResponse{
		ID:         saleID,
		StartedAt:  startedAt.Format(time.RFC3339),
		EndedAt:    endedAt.Format(time.RFC3339),
		TotalItems: req.TotalItems,
	}

	response.WriteJSON(w, http.StatusCreated, response.Success(saleResponse, "Sale created successfully"))
}
