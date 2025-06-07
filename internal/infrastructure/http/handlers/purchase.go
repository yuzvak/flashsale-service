package handlers

import (
	"net/http"

	"github.com/yuzvak/flashsale-service/internal/application/commands"
	"github.com/yuzvak/flashsale-service/internal/application/use_cases"
	"github.com/yuzvak/flashsale-service/internal/infrastructure/http/response"
	"github.com/yuzvak/flashsale-service/internal/infrastructure/monitoring"
	"github.com/yuzvak/flashsale-service/internal/pkg/logger"
)

type PurchaseHandler struct {
	purchaseUseCase *use_cases.PurchaseUseCase
	log             *logger.Logger
}

func NewPurchaseHandler(
	purchaseUseCase *use_cases.PurchaseUseCase,
	log *logger.Logger,
) *PurchaseHandler {
	return &PurchaseHandler{
		purchaseUseCase: purchaseUseCase,
		log:             log,
	}
}

func (h *PurchaseHandler) HandlePurchase() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			w.WriteHeader(http.StatusMethodNotAllowed)
			return
		}

		code := r.URL.Query().Get("code")

		h.log.Info("Purchase request received",
			"code", code,
			"method", r.Method,
			"url", r.URL.String(),
		)

		if code == "" {
			h.log.Warn("Purchase validation failed",
				"error", "checkout code is required",
				"code", code,
			)
			response.WriteValidationError(w, "Validation failed", map[string]string{
				"code": "checkout code is required",
			})
			return
		}

		cmd := commands.PurchaseCommand{
			CheckoutCode: code,
		}

		metrics := monitoring.NewPurchaseMetrics(code)
		metrics.RecordAttempt()

		handler := commands.NewPurchaseHandler(
			h.purchaseUseCase,
			h.log,
		)

		resp, err := handler.Handle(r.Context(), cmd)
		if err != nil {
			h.log.Error("Purchase command failed",
				"code", code,
				"error", err.Error(),
			)
			metrics.RecordFailure(err.Error())
			response.WriteDomainError(w, err)
			return
		}

		h.log.Info("Purchase completed",
			"code", code,
			"total_purchased", resp.TotalPurchased,
			"failed_count", resp.FailedCount,
		)

		if resp.TotalPurchased > 0 {
			metrics.RecordSuccess()
		}
		if resp.FailedCount > 0 {
			metrics.RecordFailure("Some items failed to purchase")
		}
		response.WriteSuccess(w, resp, "Purchase completed successfully")
	}
}
