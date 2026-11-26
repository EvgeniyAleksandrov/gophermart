package handler

import (
	"context"
	"errors"
	"net/http"

	"github.com/EvgeniyAleksandrov/gophermart/internal/dto"
	"github.com/EvgeniyAleksandrov/gophermart/internal/interfaces"
	"github.com/EvgeniyAleksandrov/gophermart/internal/model"
)

type Login struct {
	service AuthService
	logger  interfaces.Logger
}

func NewLogin(service AuthService, logger interfaces.Logger) *Login {
	return &Login{
		service: service,
		logger:  logger,
	}
}

// ServeHTTP validates credentials and returns a bearer token only on success.
func (h *Login) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	body := http.MaxBytesReader(w, r.Body, maxCredentialsBodyBytes)
	defer body.Close()

	var credentials dto.Credentials
	if err := credentials.Decode(body); err != nil {
		http.Error(w, "invalid request", http.StatusBadRequest)
		return
	}

	ctx, cancel := context.WithTimeout(r.Context(), httpOperationTimeout)
	defer cancel()

	token, err := h.service.Login(ctx, credentials.Login, credentials.Password)

	switch {
	case errors.Is(err, model.ErrInvalidCredentialsInput):
		http.Error(w, "invalid request", http.StatusBadRequest)
		return
	case errors.Is(err, model.ErrInvalidCredentials):
		http.Error(w, "invalid credentials", http.StatusUnauthorized)
		return
	case err != nil:
		h.logger.Error("login failed: internal error")
		http.Error(w, "internal server error", http.StatusInternalServerError)
		return
	}

	w.Header().Set("Authorization", "Bearer "+token)
	w.WriteHeader(http.StatusOK)
}
