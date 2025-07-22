package handler

import (
	"context"
	"encoding/json"
	"log/slog"
	"net/http"

	authv1 "github.com/kay-kewl/ticket-booking-system/gen/go/auth"
)

type Handler struct {
	authClient	authv1.AuthClient
	logger		*slog.Logger
}

func New(authClient authv1.AuthClient, logger *slog.Logger) *Handler {
	return &Handler{
		authClient: authClient,
		logger:		logger,
	}
}

type RegisterRequest struct {
	Email 		string `json: "email"`
	Password	string `json: "password"`
}

func (h *Handler) Register(w http.ResponseWriter, r *http.Request) {
	const op = "handler.Register"

	log := h.logger.With(slog.String("op", op))

	var req RegisterRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		log.Error("Failed to decode request body", "error", err)
		http.Error(w, "invalid request body", http.StatusBadRequest)
		return
	}

	// TODO: validate email and password

	log.Info("Making gRPC call to auth.Register", slog.String("email", req.Email))

	grpcResp, err := h.authClient.Register(context.Background(), &authv1.RegisterRequest{
		Email:		req.Email,
		Password:	req.Password,
	})

	if err != nil {
		log.Error("gRPC call failed", "error", err)
		// TODO: gRPC errors to HTTP statuses
		http.Error(w, "internal server error", http.StatusInternalServerError)
		return
	}

	log.Info("gRPC call successful", slog.Int64("userId", grpcResp.GetUserId()))

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusCreated)
	if err := json.NewEncoder(w).Encode(grpcResp); err != nil {
		log.Error("Failed to encode response", "error", err)
	}
}

type LoginRequest struct {
	Email		string `json: "email"`
	Password	string `json: "password"`
}

func (h *Handler) Login(w http.ResponseWriter, r *http.Request) {
	const op = "handler.Login"
	log := h.logger.With(slog.String("op", op))

	var req LoginRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		log.Error("Failed to decode request body", "error", err)
		http.Error(w, "invalid request body", http.StatusBadRequest)
		return
	}

	grpcResp, err := h.authClient.Login(context.Background(), &authv1.LoginRequest{
		Email:		req.Email,
		Password:	req.Password,
	})

	if err != nil {
		log.Error("gRPC call failed", "error", err)
		http.Error(w, "internal server error", http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(grpcResp)
}