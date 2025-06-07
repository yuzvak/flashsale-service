package handlers

import (
	"net/http"

	"github.com/yuzvak/flashsale-service/internal/application/commands"
	"github.com/yuzvak/flashsale-service/internal/application/ports"
	"github.com/yuzvak/flashsale-service/internal/infrastructure/http/response"
	"github.com/yuzvak/flashsale-service/internal/infrastructure/monitoring"
	"github.com/yuzvak/flashsale-service/internal/pkg/generator"
	"github.com/yuzvak/flashsale-service/internal/pkg/logger"
)

type CheckoutHandler struct {
	saleRepo     ports.SaleRepository
	checkoutRepo ports.CheckoutRepository
	cache        ports.Cache
	log          *logger.Logger
}

func NewCheckoutHandler(
	saleRepo ports.SaleRepository,
	checkoutRepo ports.CheckoutRepository,
	cache ports.Cache,
	log *logger.Logger,
) *CheckoutHandler {
	return &CheckoutHandler{
		saleRepo:     saleRepo,
		checkoutRepo: checkoutRepo,
		cache:        cache,
		log:          log,
	}
}

func (h *CheckoutHandler) HandleCheckout() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			w.WriteHeader(http.StatusMethodNotAllowed)
			return
		}

		userID := r.URL.Query().Get("user_id")
		itemID := r.URL.Query().Get("item_id")

		h.log.Info("Checkout request received",
			"user_id", userID,
			"item_id", itemID,
			"method", r.Method,
			"url", r.URL.String(),
		)

		errors := make(map[string]string)
		if userID == "" {
			errors["user_id"] = "user_id is required"
		}
		if itemID == "" {
			errors["item_id"] = "item_id is required"
		}
		if len(errors) > 0 {
			h.log.Warn("Checkout validation failed",
				"errors", errors,
				"user_id", userID,
				"item_id", itemID,
			)
			response.WriteValidationError(w, "Validation failed", errors)
			return
		}

		cmd := commands.CheckoutCommand{
			UserID: userID,
			ItemID: itemID,
		}

		metrics := monitoring.NewCheckoutMetrics(userID, itemID)
		metrics.RecordAttempt()

		handler := commands.NewCheckoutHandler(
			h.saleRepo,
			h.checkoutRepo,
			h.cache,
			h.log,
			10,
			generator.NewCodeGenerator(),
		)

		resp, err := handler.Handle(r.Context(), cmd)
		if err != nil {
			h.log.Error("Checkout command failed",
				"user_id", userID,
				"item_id", itemID,
				"error", err.Error(),
			)
			metrics.RecordFailure(err.Error())
			response.WriteDomainError(w, err)
			return
		}

		h.log.Info("Checkout completed successfully",
			"user_id", userID,
			"item_id", itemID,
			"code", resp.Code,
		)
		metrics.RecordSuccess()
		response.WriteSuccess(w, resp, "Checkout completed successfully")
	}
}
