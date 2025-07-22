package handler

import (
	"context"
	"encoding/json"
	"log/slog"
	"net/http"
	"strings"

	authv1 "github.com/kay-kewl/ticket-booking-system/gen/go/auth"
	bookingv1 "github.com/kay-kewl/ticket-booking-system/gen/go/booking"
	eventv1 "github.com/kay-kewl/ticket-booking-system/gen/go/event"
)

type Handler struct {
	authClient		authv1.AuthClient
	bookingClient	bookingv1.BookingServiceClient
	eventClient		eventv1.EventServiceClient
	logger			*slog.Logger
}

func New(authClient authv1.AuthClient, bookingClient bookingv1.BookingServiceClient, eventClient eventv1.EventServiceClient, logger *slog.Logger) *Handler {
	return &Handler{
		authClient: 	authClient,
		bookingClient:	bookingClient,
		eventClient:	eventClient,
		logger:			logger,
	}
}

type RegisterRequest struct {
	Email 		string `json:"email"`
	Password	string `json:"password"`
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
	Email		string `json:"email"`
	Password	string `json:"password"`
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

type CreateBookingRequest struct {
	EventID int64 `json:"event_id"`
	SeatIDs []int64 `json:"seat_ids"`
}

func (h *Handler) CreateBooking(w http.ResponseWriter, r *http.Request) {
	const op = "handler.CreateBooking"

	log := h.logger.With(slog.String("op", op))

	authHeader := r.Header.Get("Authorization")
	if authHeader == "" {
		http.Error(w, "missing authorization header", http.StatusUnauthorized)
	}

	headerParts := strings.Split(authHeader, " ")

	if len(headerParts) != 2 || headerParts[0] != "Bearer" {
		http.Error(w, "invalid authorization header", http.StatusUnauthorized)
		return
	}

	token := headerParts[1]

	validateResp, err := h.authClient.ValidateToken(context.Background(), &authv1.ValidateTokenRequest{Token: token})
	if err != nil {
		log.Error("Token validation failed", "error", err)
		http.Error(w, "invalid token", http.StatusUnauthorized)
		return
	}
	userID := validateResp.GetUserId()
	log.Info("Token validated successfully", slog.Int64("userID", userID))

	var req CreateBookingRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "invalid request body", http.StatusBadRequest)
		return
	}

	bookingResp, err := h.bookingClient.CreateBooking(context.Background(), &bookingv1.CreateBookingRequest{
		UserId:		userID,
		EventId:	req.EventID,
		SeatIds:	req.SeatIDs,
	})

	if err != nil {
		log.Error("gRPC call to booking-service failed", "error", err)
		http.Error(w, "failed to create booking", http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/jspn")
	w.WriteHeader(http.StatusCreated)
	json.NewEncoder(w).Encode(bookingResp)
}

func (h *Handler) ListEvents(w http.ResponseWriter, r *http.Request) {
	log := h.logger.With(slog.String("op", "handler.ListEvents"))

	grpcResp, err := h.eventClient.ListEvents(context.Background(), &eventv1.ListEventsRequest{})
	if err != nil {
		log.Error("gRPC call to event-service failed", "error", err)
		http.Error(w, "internal server error", http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(grpcResp.GetEvents())
}
