package auth

import "context"

// OAuthProvider demonstrates how an external provider would be wired.
type OAuthProvider interface {
	GetAuthURL(state string) string
	Exchange(ctx context.Context, code string) (map[string]any, error)
}

// StubOAuthProvider placeholder for docs.
type StubOAuthProvider struct{}

// GetAuthURL returns mocked URL.
func (StubOAuthProvider) GetAuthURL(state string) string {
	return "https://accounts.google.com/o/oauth2/v2/auth?state=" + state
}

// Exchange returns a basic mocked profile payload.
func (StubOAuthProvider) Exchange(ctx context.Context, code string) (map[string]any, error) {
	return map[string]any{
		"email":       "demo-google@kidpech.app",
		"name":        "Google Demo",
		"external_id": code,
	}, nil
}
