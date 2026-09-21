package auth

import (
	"context"
	"errors"
	"testing"
)

func TestMemoryVerifier_VerifyToken(t *testing.T) {
	v := NewMemoryVerifier()

	t.Run("non-empty token resolves to itself as sub", func(t *testing.T) {
		identity, err := v.VerifyToken(context.Background(), "some-token")
		if err != nil {
			t.Fatalf("VerifyToken returned error: %v", err)
		}
		if identity.Sub != "some-token" {
			t.Errorf("Sub = %q, want %q", identity.Sub, "some-token")
		}
	})

	t.Run("empty token is rejected", func(t *testing.T) {
		_, err := v.VerifyToken(context.Background(), "")
		if !errors.Is(err, ErrInvalidToken) {
			t.Errorf("VerifyToken error = %v, want %v", err, ErrInvalidToken)
		}
	})
}
