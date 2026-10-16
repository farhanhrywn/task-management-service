// Package jwtutil signs and verifies HS256 access tokens.
package jwtutil

import (
	"errors"
	"time"

	"github.com/golang-jwt/jwt/v5"
	"github.com/google/uuid"

	"github.com/farhanhrywn/task-management-service/pkg/clock"
)

var ErrInvalidToken = errors.New("invalid token")

type Claims struct {
	Role string `json:"role"`
	jwt.RegisteredClaims
}

type Manager struct {
	secret []byte
	ttl    time.Duration
	clock  clock.Clock
}

func NewManager(secret string, ttl time.Duration, clk clock.Clock) *Manager {
	return &Manager{secret: []byte(secret), ttl: ttl, clock: clk}
}

func (m *Manager) TTL() time.Duration { return m.ttl }

// Generate returns a signed token with sub, role, iat and exp claims.
func (m *Manager) Generate(userID uuid.UUID, role string) (string, error) {
	now := m.clock.Now()
	claims := Claims{
		Role: role,
		RegisteredClaims: jwt.RegisteredClaims{
			Subject:   userID.String(),
			IssuedAt:  jwt.NewNumericDate(now),
			ExpiresAt: jwt.NewNumericDate(now.Add(m.ttl)),
		},
	}
	return jwt.NewWithClaims(jwt.SigningMethodHS256, claims).SignedString(m.secret)
}

// Parse verifies signature, algorithm and expiry, and returns the subject and role.
func (m *Manager) Parse(token string) (uuid.UUID, string, error) {
	var claims Claims
	_, err := jwt.ParseWithClaims(token, &claims,
		func(*jwt.Token) (any, error) { return m.secret, nil },
		jwt.WithValidMethods([]string{jwt.SigningMethodHS256.Alg()}),
		jwt.WithExpirationRequired(),
		jwt.WithIssuedAt(),
		jwt.WithTimeFunc(m.clock.Now),
	)
	if err != nil {
		return uuid.Nil, "", errors.Join(ErrInvalidToken, err)
	}
	userID, err := uuid.Parse(claims.Subject)
	if err != nil || claims.Role == "" {
		return uuid.Nil, "", ErrInvalidToken
	}
	return userID, claims.Role, nil
}
