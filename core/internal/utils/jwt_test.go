package utils

import (
	"testing"

	"github.com/luketaylor45/atlas/core/internal/config"
)

const testSecret = "test-secret-test-secret-test-secret-0000"

func TestConsoleTicketIsBoundToItsService(t *testing.T) {
	config.AppConfig.JWTSecret = testSecret

	ticket, err := GenerateConsoleTicket(7, "service-a")
	if err != nil {
		t.Fatal(err)
	}
	claims, err := ValidateConsoleTicket(ticket, "service-a")
	if err != nil || claims.UserID != 7 {
		t.Fatalf("valid ticket rejected: %v", err)
	}
	if _, err := ValidateConsoleTicket(ticket, "service-b"); err == nil {
		t.Fatal("ticket was accepted for a different service")
	}
}

func TestSessionAndConsoleTokensAreNotInterchangeable(t *testing.T) {
	config.AppConfig.JWTSecret = testSecret

	ticket, _ := GenerateConsoleTicket(7, "service-a")
	if _, err := ValidateToken(ticket); err == nil {
		t.Fatal("console ticket was accepted as a session token")
	}

	session, _ := GenerateToken(7, true)
	if _, err := ValidateToken(session); err != nil {
		t.Fatalf("session token rejected: %v", err)
	}
	if _, err := ValidateConsoleTicket(session, "service-a"); err == nil {
		t.Fatal("session token was accepted as a console ticket")
	}
}

func TestTokensSignedWithAnotherSecretAreRejected(t *testing.T) {
	config.AppConfig.JWTSecret = "change-me-in-production"
	forged, _ := GenerateToken(1, true)

	config.AppConfig.JWTSecret = testSecret
	if _, err := ValidateToken(forged); err == nil {
		t.Fatal("token signed with the old default secret was accepted")
	}
}
