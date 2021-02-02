package helpers

import (
	"bitbucket.org/calmisland/go-server-auth/authmiddlewares"
	"github.com/getsentry/sentry-go"
	sentryecho "github.com/getsentry/sentry-go/echo"
	"github.com/labstack/echo/v4"
)

// GetAccountID will extract an AccountID from a JWT
func GetAccountID(c echo.Context) string {
	cc := c.(*authmiddlewares.AuthContext)
	accountID := cc.Session.Data.AccountID

	hub := sentryecho.GetHubFromContext(c)
	hub.ConfigureScope(func(scope *sentry.Scope) {
		scope.SetUser(sentry.User{
			ID: accountID,
		})
	})
	return accountID
}
