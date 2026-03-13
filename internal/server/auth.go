package server

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"strings"
	"sync"
	"time"

	"github.com/golang-jwt/jwt/v5"
	"golang.org/x/crypto/bcrypt"
)

const BcryptCost = 12

var (
	ErrInvalidCredentials = errors.New("invalid credentials")
	ErrUnauthorized       = errors.New("unauthorized")
)

type Auth struct {
	mu        sync.RWMutex
	jwtSecret []byte
	blocklist map[string]time.Time
}

func NewAuth(secret []byte) *Auth {
	return &Auth{jwtSecret: secret, blocklist: make(map[string]time.Time)}
}

func (a *Auth) UpdateSecret(secret []byte) {
	a.mu.Lock()
	a.jwtSecret = secret
	a.mu.Unlock()
}

func (a *Auth) getSecret() []byte {
	a.mu.RLock()
	s := a.jwtSecret
	a.mu.RUnlock()
	return s
}

func HashPassword(password string) (string, error) {
	hash, err := bcrypt.GenerateFromPassword([]byte(password), BcryptCost)
	if err != nil {
		return "", fmt.Errorf("hash password: %w", err)
	}
	return string(hash), nil
}

func CheckPassword(hash, password string) bool {
	return bcrypt.CompareHashAndPassword([]byte(hash), []byte(password)) == nil
}

func (a *Auth) GenerateToken(subject string, duration time.Duration) (string, error) {
	claims := jwt.RegisteredClaims{
		Subject:   subject,
		IssuedAt:  jwt.NewNumericDate(time.Now()),
		ExpiresAt: jwt.NewNumericDate(time.Now().Add(duration)),
	}

	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	return token.SignedString(a.getSecret())
}

func (a *Auth) ValidateToken(tokenString string) (*jwt.RegisteredClaims, error) {
	a.mu.RLock()
	if _, blocked := a.blocklist[tokenString]; blocked {
		a.mu.RUnlock()
		return nil, ErrUnauthorized
	}
	a.mu.RUnlock()

	secret := a.getSecret()
	token, err := jwt.ParseWithClaims(tokenString, &jwt.RegisteredClaims{}, func(t *jwt.Token) (any, error) {
		if _, ok := t.Method.(*jwt.SigningMethodHMAC); !ok {
			return nil, fmt.Errorf("unexpected signing method: %v", t.Header["alg"])
		}
		return secret, nil
	})
	if err != nil {
		return nil, ErrUnauthorized
	}

	claims, ok := token.Claims.(*jwt.RegisteredClaims)
	if !ok || !token.Valid {
		return nil, ErrUnauthorized
	}

	return claims, nil
}

func (a *Auth) BlockToken(tokenString string, expiresAt time.Time) {
	a.mu.Lock()
	a.blocklist[tokenString] = expiresAt
	// Prune expired entries
	now := time.Now()
	for k, exp := range a.blocklist {
		if now.After(exp) {
			delete(a.blocklist, k)
		}
	}
	a.mu.Unlock()
}

type contextKey string

const claimsKey contextKey = "claims"

func (a *Auth) Middleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var tokenString string

		// Try cookie first (HttpOnly), then Authorization header
		if c, err := r.Cookie("burrow_token"); err == nil && c.Value != "" {
			tokenString = c.Value
		} else {
			authHeader := r.Header.Get("Authorization")
			if authHeader == "" {
				http.Error(w, `{"error":"missing authorization"}`, http.StatusUnauthorized)
				return
			}
			tokenString = strings.TrimPrefix(authHeader, "Bearer ")
			if tokenString == authHeader {
				http.Error(w, `{"error":"invalid authorization format"}`, http.StatusUnauthorized)
				return
			}
		}

		claims, err := a.ValidateToken(tokenString)
		if err != nil {
			http.Error(w, `{"error":"invalid or expired token"}`, http.StatusUnauthorized)
			return
		}

		ctx := context.WithValue(r.Context(), claimsKey, claims)
		ctx = context.WithValue(ctx, tokenKey, tokenString)
		next.ServeHTTP(w, r.WithContext(ctx))
	})
}

const tokenKey contextKey = "token"
