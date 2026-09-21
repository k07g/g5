package auth

import "context"

// Identity is the caller identity resolved from an access token.
type Identity struct {
	Sub   string
	Email string
}

// Verifier resolves the caller identity for a bearer access token. This
// service does not issue or manage credentials itself — that is
// github.com/k07g/g4's responsibility — so Verifier only covers the
// read-only "who does this token belong to" question needed to scope
// career sheet data per user.
type Verifier interface {
	VerifyToken(ctx context.Context, accessToken string) (*Identity, error)
}
