package auth

import (
	"context"
	"errors"
)

var ErrInvalidToken = errors.New("invalid or expired access token")

// MemoryVerifier is a local-development stand-in for CognitoVerifier that
// requires no AWS account. It treats any non-empty bearer token as valid
// and uses the token itself as the caller's identity (sub).
//
// This intentionally does not try to validate tokens against g4
// (github.com/k07g/g4): when g4 runs with its own AUTH_PROVIDER=memory, it
// issues opaque per-session tokens rather than real Cognito access tokens,
// which this service has no way to verify independently. Using the token
// as the sub keeps career sheet data consistently scoped to the same
// signed-in session for local development, without either service needing
// to trust the other's internal state. Not safe for production use: it
// performs no real authentication.
type MemoryVerifier struct{}

func NewMemoryVerifier() *MemoryVerifier {
	return &MemoryVerifier{}
}

var _ Verifier = (*MemoryVerifier)(nil)

func (m *MemoryVerifier) VerifyToken(_ context.Context, accessToken string) (*Identity, error) {
	if accessToken == "" {
		return nil, ErrInvalidToken
	}
	return &Identity{Sub: accessToken}, nil
}
