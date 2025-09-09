package handler

import (
	"encoding/json"
	"log/slog"
	"net/http"
	"strconv"
	"strings"

	authv1 "github.com/kay-kewl/ticket-booking-system/gen/go/auth"
	bookingv1 "github.com/kay-kewl/ticket-booking-system/gen/go/booking"
	eventv1 "github.com/kay-kewl/ticket-booking-system/gen/go/event"

	"github.com/go-playground/validator/v10"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

var validate = validator.New()

type Handler struct {
	authClient    authv1.AuthClient
	bookingClient bookingv1.BookingServiceClient
	eventClient   eventv1.EventServiceClient
	logger        *slog.Logger
	webhookSecret []byte
}

func New(authClient authv1.AuthClient, bookingClient bookingv1.BookingServiceClient, eventClient eventv1.EventServiceClient, webhookSecret string, logger *slog.Logger) *Handler {
	return &Handler{
		authClient:    authClient,
		bookingClient: bookingClient,
		eventClient:   eventClient,
		logger:        logger,
		webhookSecret: []byte(webhookSecret),
	}
}

type RegisterRequest struct {
	Email    string `json:"email" validate:"required,email"`
	Password string `json:"password" validate:"required,min=8"`
}

func (h *Handler) Register(w http.ResponseWriter, r *http.Request) {
	const op = "handler.Register"

	log := h.logger.With(slog.String("op", op))
	r.Body = http.MaxBytesReader(w, r.Body, 1024*1024)

	var req RegisterRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		log.ErrorContext(r.Context(), "Failed to decode request body", "error", err)
		http.Error(w, "invalid request body", http.StatusBadRequest)
		return
	}

	// TODO: validate email and password
	if err := validate.Struct(req); err != nil {
		log.WarnContext(r.Context(), "Invalid request body for registration", "error", err)
		http.Error(w, "invalid request: "+err.Error(), http.StatusBadRequest)
		return
	}

	log.Info("Making gRPC call to auth.Register", slog.String("email", req.Email))

	grpcResp, err := h.authClient.Register(r.Context(), &authv1.RegisterRequest{
		Email:    req.Email,
		Password: req.Password,
	})

	if err != nil {
		// TODO: gRPC errors to HTTP statuses
		if st, ok := status.FromError(err); ok {
			switch st.Code() {
			case codes.InvalidArgument:
				log.WarnContext(r.Context(), "Invalid argument for registration", "email", req.Email, "error", st.Message())
				http.Error(w, st.Message(), http.StatusBadRequest)
				return
			case codes.AlreadyExists:
				log.WarnContext(r.Context(), "Attempt to register existing user", "email", req.Email)
				http.Error(w, "user with this email already exists", http.StatusConflict)
				return
			default:
				log.ErrorContext(r.Context(), "gRPC call failed with unhandled status", "status", st.Code(), "error", err)
				http.Error(w, "internal server error", http.StatusInternalServerError)
				return
			}
		}
		log.ErrorContext(r.Context(), "gRPC call failed", "error", err)
		http.Error(w, "internal server error", http.StatusInternalServerError)
		return
	}

	log.Info("gRPC call successful", slog.Int64("userId", grpcResp.GetUserId()))

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusCreated)
	if err := json.NewEncoder(w).Encode(grpcResp); err != nil {
		log.ErrorContext(r.Context(), "Failed to encode response", "error", err)
	}
}

type LoginRequest struct {
	Email    string `json:"email" validate:"required,email"`
	Password string `json:"password" validate:"required"`
}

func (h *Handler) Login(w http.ResponseWriter, r *http.Request) {
	const op = "handler.Login"
	log := h.logger.With(slog.String("op", op))
	r.Body = http.MaxBytesReader(w, r.Body, 1024*1024)

	var req LoginRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		log.ErrorContext(r.Context(), "Failed to decode request body", "error", err)
		http.Error(w, "invalid request body", http.StatusBadRequest)
		return
	}

	if err := validate.Struct(req); err != nil {
		log.WarnContext(r.Context(), "Invalid request for body login", "error", err)
		http.Error(w, "invalid request: "+err.Error(), http.StatusBadRequest)
		return
	}

	grpcResp, err := h.authClient.Login(r.Context(), &authv1.LoginRequest{
		Email:    req.Email,
		Password: req.Password,
	})

	if err != nil {
		log.ErrorContext(r.Context(), "gRPC call failed", "error", err)
		http.Error(w, "internal server error", http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(grpcResp)
}

type CreateBookingRequest struct {
	EventID int64   `json:"event_id" validate:"required"`
	SeatIDs []int64 `json:"seat_ids" validate:"required"`
}

func (h *Handler) CreateBooking(w http.ResponseWriter, r *http.Request) {
	const op = "handler.CreateBooking"

	log := h.logger.With(slog.String("op", op))
	r.Body = http.MaxBytesReader(w, r.Body, 1024*1024)

	authHeader := r.Header.Get("Authorization")
	if authHeader == "" {
		http.Error(w, "missing authorization header", http.StatusUnauthorized)
		return
	}

	headerParts := strings.Split(authHeader, " ")

	if len(headerParts) != 2 || headerParts[0] != "Bearer" {
		http.Error(w, "invalid authorization header", http.StatusUnauthorized)
		return
	}

	token := headerParts[1]

	validateResp, err := h.authClient.ValidateToken(r.Context(), &authv1.ValidateTokenRequest{Token: token})
	if err != nil {
		log.ErrorContext(r.Context(), "Token validation failed", "error", err)
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

	if err := validate.Struct(req); err != nil {
		log.WarnContext(r.Context(), "Invalid request for body create booking", "error", err)
		http.Error(w, "invalid request: "+err.Error(), http.StatusBadRequest)
		return
	}

	bookingResp, err := h.bookingClient.CreateBooking(r.Context(), &bookingv1.CreateBookingRequest{
		UserId:  userID,
		EventId: req.EventID,
		SeatIds: req.SeatIDs,
	})

	if err != nil {
		if st, ok := status.FromError(err); ok {
			switch st.Code() {
			case codes.FailedPrecondition:
				if strings.Contains(st.Message(), "payment failed") {
					log.WarnContext(r.Context(), "Booking failed due to payment error", "userID", userID, "error", st.Message())
					http.Error(w, "Payment failed", http.StatusConflict)
					return
				}
				log.WarnContext(r.Context(), "Attempt to book reserved seats", "userID", userID, "seats", req.SeatIDs, "error", st.Message())
				http.Error(w, "booked seats have already been reserved", http.StatusConflict)
				return
			default:
				log.ErrorContext(r.Context(), "Unhandled gRPC error from booking-service", "userID", userID, "code", st.Code(), "error", st.Message())
				http.Error(w, "Failed to create booking due to an internal error", http.StatusInternalServerError)
				return
			}
		}
		log.ErrorContext(r.Context(), "gRPC call to booking-service failed", "error", err)
		http.Error(w, "failed to create booking", http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusCreated)
	json.NewEncoder(w).Encode(bookingResp)
}

func (h *Handler) ListEvents(w http.ResponseWriter, r *http.Request) {
	log := h.logger.With(slog.String("op", "handler.ListEvents"))

	log.InfoContext(r.Context(), "request received")

	pageStr := r.URL.Query().Get("page")
	if pageStr == "" {
		pageStr = "1"
	}
	page, err := strconv.Atoi(pageStr)
	if err != nil || page < 1 {
		log.WarnContext(r.Context(), "Invalid page parameter. Must be a positive integer", "value", pageStr, "error", err)
		http.Error(w, "Invalid page parameter. Must be a positive integer", http.StatusBadRequest)
		return
	}

	sizeStr := r.URL.Query().Get("size")
	if sizeStr == "" {
		sizeStr = "10"
	}
	size, err := strconv.Atoi(sizeStr)
	if err != nil || size < 1 {
		log.WarnContext(r.Context(), "Invalid size parameter. Must be a positive integer", "value", sizeStr, "error", err)
		http.Error(w, "Invalid size parameter. Must be a positive integer", http.StatusBadRequest)
		return
	}

	grpcResp, err := h.eventClient.ListEvents(r.Context(), &eventv1.ListEventsRequest{
		PageNumber: int32(page),
		PageSize:   int32(size),
	})
	if err != nil {
		log.ErrorContext(r.Context(), "gRPC call to event-service failed", "error", err)
		http.Error(w, "internal server error", http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(grpcResp)
}

type paymentWebhookPayload struct {
	BookingID 	int64 	`json:"booking_id"`
	Status 		string 	`json:"status"`
	Timestamp	string 	`json:"timestamp"`
}

func (h *Handler) PaymentWebhook(w http.ResponseWriter, r *http.Request) {
	body, err := io.ReadAll(r.Body)
	if err != nil {
		http.Error(w, "Cannot read body", http.StatusBadRequest)
		return
	}

	receivedSig := r.Header.Get("X-Webhook-Signature")
	if !h.isValidSignature(body, receivedSig) {
		http.Error(w, "Invalid signature", http.StatusForbidden)
		return
	}

	var payload paymentWebhookPayload
	if err := json.Unmarshal(body, &payload); err != nil {
		http.Error(w, "Invalid payload", http.StatusBadRequest)
		return
	}

	ts, err := time.Parse(time.RFC3339, payload.Timestamp)
	if err != nil {
		http.Error(w, "Invalid timestamp format", http.StatusBadRequest)
		return
	}

	if time.Since(ts) > 5*time.Minute {
		http.Error(w, "Webhook timestamp is too old", http.StatusForbidden)
		return
	}

	ctx, cancel := context.WithTimeout(r.Context(), 1*time.Minute)
	defer cancel()

	_, err := h.bookingClient.HandlePaymentWebhook(ctx, &bookingv1.HandlePaymentWebhookRequest{
		BookingId: 	payload.BookingID,
		Status: 	payload.Status,
	})

	if err != nil {
		if st, ok := status.FromError(err); ok && st.Code() == codes.InvalidArgument {
			http.Error(w, st.Message(), http.StatusBadRequest)
			return
		}
		http.Error(w, "Upstream service error", http.StatusServiceUnavailable)
		return
	}

	w.WriteHeader(http.StatusOK)
}

func (h *Handler) isValidSignature(body []byte, receivedSignature string) bool {
	mac := hmac.New(sha256.New, h.webhookSecret)
	mac.Write(body)
	expectedSignature := hex.EncodeToString(mac.Sum(nil))

	return subtle.ConstantTimeCompare([]byte(receivedSignature), []byte(expectedSignature)) == 1
}