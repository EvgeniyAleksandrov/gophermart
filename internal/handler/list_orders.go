package handler

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"

	"github.com/EvgeniyAleksandrov/gophermart/internal/interfaces"
	"github.com/EvgeniyAleksandrov/gophermart/internal/model"
	"github.com/EvgeniyAleksandrov/gophermart/internal/requestcontext"
	"github.com/EvgeniyAleksandrov/gophermart/internal/service/mapper"
)

type ListOrders struct {
	service     OrderService
	mapper      *mapper.Order
	userContext *requestcontext.User
	logger      interfaces.Logger
}

func NewListOrders(service OrderService, logger interfaces.Logger) *ListOrders {
	return &ListOrders{
		service:     service,
		mapper:      mapper.NewOrder(),
		userContext: requestcontext.NewUser(),
		logger:      logger,
	}
}

// ServeHTTP returns the user's orders as JSON, or an empty 204 response.
// Presentation completes before headers are written; JWTs are never refreshed here.
func (h *ListOrders) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	userID, ok := h.userContext.ID(r.Context())
	if !ok || userID <= 0 {
		http.Error(w, "unauthorized", http.StatusUnauthorized)
		return
	}

	ctx, cancel := context.WithTimeout(r.Context(), httpOperationTimeout)
	defer cancel()

	orders, err := h.service.List(ctx, userID)
	if errors.Is(err, model.ErrUserNotFound) {
		http.Error(w, "unauthorized", http.StatusUnauthorized)
		return
	}

	if err != nil {
		h.logger.Error("list orders failed: internal error")
		http.Error(w, "internal server error", http.StatusInternalServerError)
		return
	}

	if len(orders) == 0 {
		w.WriteHeader(http.StatusNoContent)
		return
	}

	response, err := h.mapper.Present(orders)
	if err != nil {
		h.logger.Error("present orders failed: internal error")
		http.Error(w, "internal server error", http.StatusInternalServerError)
		return
	}

	body, err := json.Marshal(response)
	if err != nil {
		h.logger.Error("encode orders failed: internal error")
		http.Error(w, "internal server error", http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	if _, err := w.Write(body); err != nil {
		h.logger.Error("write orders response failed")
	}
}
