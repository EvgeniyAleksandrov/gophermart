package handler

import (
	"context"
	"errors"
	"net/http"

	"github.com/EvgeniyAleksandrov/gophermart/internal/dto"
	"github.com/EvgeniyAleksandrov/gophermart/internal/interfaces"
	"github.com/EvgeniyAleksandrov/gophermart/internal/model"
)

// Register handles POST /api/user/register.
type Register struct {
	service AuthService
	logger  interfaces.Logger
}

// NewRegister receives the authentication service and shared logger.
func NewRegister(service AuthService, logger interfaces.Logger) *Register {
	return &Register{
		service: service,
		logger:  logger,
	}
}

// ServeHTTP validates credentials and returns a bearer token only on success.
func (h *Register) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	body := http.MaxBytesReader(w, r.Body, maxCredentialsBodyBytes)
	defer body.Close()

	var credentials dto.Credentials
	if err := credentials.Decode(body); err != nil {
		http.Error(w, "invalid request", http.StatusBadRequest)
		return
	}

	ctx, cancel := context.WithTimeout(r.Context(), httpOperationTimeout)
	defer cancel()

	token, err := h.service.Register(ctx, credentials.Login, credentials.Password)
	switch {
	case errors.Is(err, model.ErrInvalidCredentialsInput):
		http.Error(w, "invalid request", http.StatusBadRequest)
		return
	case errors.Is(err, model.ErrLoginTaken):
		http.Error(w, "login already taken", http.StatusConflict)
		return
	case err != nil:
		h.logger.Error("register failed: internal error")
		http.Error(w, "internal server error", http.StatusInternalServerError)
		return
	}

	w.Header().Set("Authorization", "Bearer "+token)
	w.WriteHeader(http.StatusOK)
}
