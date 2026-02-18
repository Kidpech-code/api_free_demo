package middleware

import (
	"github.com/gofiber/fiber/v2"
)

const githubSessionCookie = "_gh_sess"

// SessionVerifier is a function that validates a session cookie value and returns
// the authenticated login name.  Satisfied by GitHubAuthHandler.VerifySession.
type SessionVerifier func(cookieValue string) (login string, ok bool)

// RequireGitHubSession protects a route so that only browsers carrying a valid
// signed GitHub session cookie may proceed.  All others are redirected to the
// GitHub OAuth login page.
func RequireGitHubSession(verifier SessionVerifier) fiber.Handler {
	return func(c *fiber.Ctx) error {
		cookieVal := c.Cookies(githubSessionCookie)
		if cookieVal == "" {
			return c.Redirect("/login.html", fiber.StatusFound)
		}
		login, ok := verifier(cookieVal)
		if !ok || login == "" {
			// tampered / expired cookie → clear it and show login page
			c.ClearCookie(githubSessionCookie)
			return c.Redirect("/login.html", fiber.StatusFound)
		}
		c.Locals("github_login", login)
		return c.Next()
	}
}
