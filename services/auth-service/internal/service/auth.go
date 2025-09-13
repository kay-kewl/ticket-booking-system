package service

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/golang-jwt/jwt/v5"
	"golang.org/x/crypto/bcrypt"

	"github.com/kay-kewl/ticket-booking-system/services/auth-service/internal/storage"
)

var ErrUserExists = errors.New("user already exists")
var ErrInvalidCredentials = errors.New("invalid credentials")

type UserSaver interface {
	SaveUser(ctx context.Context, email string, passHash []byte) (int64, error)
}

type UserProvider interface {
	User(ctx context.Context, email string) (id int64, passHash []byte, err error)
    UserDetails(ctx context.Context, userID int64) (email string, err error)
}

type Auth struct {
	jwtSecret    []byte
	tokenTTL     time.Duration
	userProvider UserProvider
	userSaver    UserSaver
}

func New(jwtSecret string, tokenTTL time.Duration, userProvider UserProvider, userSaver UserSaver) *Auth {
	return &Auth{
		jwtSecret:    []byte(jwtSecret),
		tokenTTL:     tokenTTL,
		userProvider: userProvider,
		userSaver:    userSaver,
	}
}

func (a *Auth) Register(ctx context.Context, email string, password string) (int64, error) {
	const op = "Auth.Register"

	// TODO: validate email and password

	passHash, err := bcrypt.GenerateFromPassword([]byte(password), bcrypt.DefaultCost)
	if err != nil {
		return 0, fmt.Errorf("%s: %w", op, err)
	}

	id, err := a.userSaver.SaveUser(ctx, email, passHash)
	if err != nil {
		if errors.Is(err, storage.ErrUserExists) {
			return 0, fmt.Errorf("%s: %w", op, ErrUserExists)
		}
		return 0, fmt.Errorf("%s: %w", op, err)
	}

	return id, nil
}

func (a *Auth) Login(ctx context.Context, email string, password string) (string, error) {
	const op = "Auth.Login"

	// TODO: validate

	id, passHash, err := a.userProvider.User(ctx, email)
	if err != nil {
		// TODO: not found error
		if errors.Is(err, storage.ErrUserNotFound) {
			return "", fmt.Errorf("%s: %w", op, ErrInvalidCredentials)
		}
		return "", fmt.Errorf("%s: %w", op, err)
	}

	if err := bcrypt.CompareHashAndPassword(passHash, []byte(password)); err != nil {
		// TODO: wrong password error
		return "", fmt.Errorf("%s: %w", op, ErrInvalidCredentials)
	}

	token := jwt.NewWithClaims(jwt.SigningMethodHS256, jwt.MapClaims{
		"uid": id,
		"exp": time.Now().Add(a.tokenTTL).Unix(),
	})

	// TODO: hide the secret
	tokenString, err := token.SignedString(a.jwtSecret)
	if err != nil {
		return "", fmt.Errorf("%s: %w", op, err)
	}

	return tokenString, nil
}

func (a *Auth) ValidateToken(ctx context.Context, tokenString string) (int64, error) {
	const op = "Auth.ValidateToken"

	token, err := jwt.Parse(tokenString, func(token *jwt.Token) (interface{}, error) {
		if _, ok := token.Method.(*jwt.SigningMethodHMAC); !ok {
			return nil, fmt.Errorf("Unexpected signing method: %v", token.Header["alg"])
		}

		return a.jwtSecret, nil
	})

	if err != nil {
		return 0, fmt.Errorf("%s: %w", op, err)
	}

	if claims, ok := token.Claims.(jwt.MapClaims); ok && token.Valid {
		if uidFloat, ok := claims["uid"].(float64); ok {
			return int64(uidFloat), nil
		}
	}

	return 0, fmt.Errorf("%s: invalid token", op)
}

func (a *Auth) GetUserDetails(ctx context.Context, userID int64) (string, error) {
    const op = "Auth.GetUserDetails"

    email, err := a.userProvider.UserDetails(ctx, userID)
    if err != nil {
        if errors.Is(err, storage.ErrUserNotFound) {
            return "", fmt.Errorf("%s: %w", op, err)
        }
        return "", fmt.Errorf("%s: %w", op, err)
    }

    return email, nil
}
