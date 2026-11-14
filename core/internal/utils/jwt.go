package utils

import (
	"errors"
	"time"

	"github.com/golang-jwt/jwt/v5"
	"github.com/luketaylor45/atlas/core/internal/config"
)

const (
	sessionAudience = "atlas-session"
	consoleAudience = "atlas-console"

	consoleTicketLifetime = 30 * time.Second
)

type Claims struct {
	UserID  uint `json:"user_id"`
	IsAdmin bool `json:"is_admin"`
	jwt.RegisteredClaims
}

// ConsoleClaims authorise one websocket connection to one service's console.
type ConsoleClaims struct {
	UserID      uint   `json:"user_id"`
	ServiceUUID string `json:"service_uuid"`
	jwt.RegisteredClaims
}

func signingKey(token *jwt.Token) (interface{}, error) {
	if _, ok := token.Method.(*jwt.SigningMethodHMAC); !ok {
		return nil, errors.New("unexpected signing method")
	}
	return []byte(config.AppConfig.JWTSecret), nil
}

func GenerateToken(userID uint, isAdmin bool) (string, error) {
	claims := &Claims{
		UserID:  userID,
		IsAdmin: isAdmin,
		RegisteredClaims: jwt.RegisteredClaims{
			ExpiresAt: jwt.NewNumericDate(time.Now().Add(24 * time.Hour)),
			Issuer:    "atlas-core",
			Audience:  jwt.ClaimStrings{sessionAudience},
		},
	}

	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	return token.SignedString([]byte(config.AppConfig.JWTSecret))
}

func ValidateToken(tokenString string) (*Claims, error) {
	claims := &Claims{}
	token, err := jwt.ParseWithClaims(tokenString, claims, signingKey, jwt.WithValidMethods([]string{"HS256"}))
	if err != nil {
		return nil, err
	}
	if !token.Valid {
		return nil, errors.New("invalid token")
	}
	// Console tickets are signed with the same key and must never work as a session.
	for _, audience := range claims.Audience {
		if audience != sessionAudience {
			return nil, errors.New("invalid token audience")
		}
	}
	return claims, nil
}

// GenerateConsoleTicket issues a short-lived, single-purpose ticket. Browsers
// cannot send an Authorization header on a websocket, so the ticket travels in
// the URL and is useless after a few seconds or for any other service.
func GenerateConsoleTicket(userID uint, serviceUUID string) (string, error) {
	claims := &ConsoleClaims{
		UserID:      userID,
		ServiceUUID: serviceUUID,
		RegisteredClaims: jwt.RegisteredClaims{
			ExpiresAt: jwt.NewNumericDate(time.Now().Add(consoleTicketLifetime)),
			Issuer:    "atlas-core",
			Audience:  jwt.ClaimStrings{consoleAudience},
		},
	}
	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	return token.SignedString([]byte(config.AppConfig.JWTSecret))
}

func ValidateConsoleTicket(ticket string, serviceUUID string) (*ConsoleClaims, error) {
	claims := &ConsoleClaims{}
	token, err := jwt.ParseWithClaims(ticket, claims, signingKey,
		jwt.WithValidMethods([]string{"HS256"}),
		jwt.WithAudience(consoleAudience),
		jwt.WithExpirationRequired(),
	)
	if err != nil {
		return nil, err
	}
	if !token.Valid || claims.ServiceUUID == "" || claims.ServiceUUID != serviceUUID {
		return nil, errors.New("invalid console ticket")
	}
	return claims, nil
}
