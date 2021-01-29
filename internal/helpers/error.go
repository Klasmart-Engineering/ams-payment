package helpers

import (
	"bitbucket.org/calmisland/go-server-requests/apierrors"

	"github.com/getsentry/sentry-go"
	sentryecho "github.com/getsentry/sentry-go/echo"
	"github.com/labstack/echo/v4"
)

func HandleInternalError(c echo.Context, err error) error {

	if hub := sentryecho.GetHubFromContext(c); hub != nil {
		hub.CaptureException(err)
	} else {
		sentry.CaptureException(err)
	}

	// // Log the server error
	// errorContext := getErrorContext(response.Request)
	// logger.LogError(err, errorContext)

	apiErr := apierrors.ErrorInternalServerError
	errorMessage := err.Error()
	apiErr = apiErr.WithMessage(errorMessage)

	return c.JSON(apiErr.StatusCode, apiErr)
}
