package auth

import (
	"testing"
	"time"

	"github.com/aniketkr01/workflow-engine/internal/domain"
	"github.com/google/uuid"
)

func TestJWTManager_GenerateAndVerify(t *testing.T) {
	manager := NewJWTManager("test-secret", time.Minute)
	userID := uuid.New()
	user := &domain.User{
		ID:    userID,
		Email: "user@example.com",
		Role:  domain.RoleViewer,
	}

	token, err := manager.Generate(user)
	if err != nil {
		t.Fatalf("Generate() error = %v", err)
	}

	claims, err := manager.Verify(token)
	if err != nil {
		t.Fatalf("Verify() error = %v", err)
	}

	if claims.UserID != userID.String() {
		t.Fatalf("expected user id %q, got %q", userID.String(), claims.UserID)
	}
	if claims.Email != user.Email {
		t.Fatalf("expected email %q, got %q", user.Email, claims.Email)
	}
	if claims.Role != user.Role {
		t.Fatalf("expected role %q, got %q", user.Role, claims.Role)
	}
}

func TestJWTManager_VerifyFailures(t *testing.T) {
	manager := NewJWTManager("test-secret", -time.Minute)
	user := &domain.User{
		ID:    uuid.New(),
		Email: "user@example.com",
		Role:  domain.RoleViewer,
	}

	expiredToken, err := manager.Generate(user)
	if err != nil {
		t.Fatalf("Generate expired token error = %v", err)
	}

	tests := []struct {
		name      string
		token     string
		wantError error
	}{
		{
			name:      "invalid token format",
			token:     "not-a-token",
			wantError: ErrInvalidToken,
		},
		{
			name:      "expired token",
			token:     expiredToken,
			wantError: ErrExpiredToken,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := manager.Verify(tt.token)
			if err == nil {
				t.Fatalf("expected error, got nil")
			}
			if err != tt.wantError {
				t.Fatalf("expected %v, got %v", tt.wantError, err)
			}
		})
	}
}

func TestClaims_GetUserID(t *testing.T) {
	id := uuid.New()
	claims := &Claims{UserID: id.String()}

	got, err := claims.GetUserID()
	if err != nil {
		t.Fatalf("GetUserID() error = %v", err)
	}
	if got != id {
		t.Fatalf("expected %v, got %v", id, got)
	}
}
