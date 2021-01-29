package routers

import (
	"bitbucket.org/calmisland/go-server-auth/authmiddlewares"
	apiControllerV1 "bitbucket.org/calmisland/payment-lambda-funcs/internal/controllers/v1"
	apiControllerV2 "bitbucket.org/calmisland/payment-lambda-funcs/internal/controllers/v2"
	"bitbucket.org/calmisland/payment-lambda-funcs/internal/global"
	sentryecho "github.com/getsentry/sentry-go/echo"
	"github.com/labstack/echo/v4"
	"github.com/labstack/echo/v4/middleware"
)

// SetupRouter is ...
func SetupRouter() *echo.Echo {
	// Echo instance
	e := echo.New()

	authMiddleware := authmiddlewares.EchoAuthMiddleware(global.AccessTokenValidator, true)

	// Middleware
	e.Use(middleware.Logger())
	e.Use(middleware.Recover())
	e.Use(sentryecho.New(sentryecho.Options{}))

	// e.Use(func(next echo.HandlerFunc) echo.HandlerFunc {
	// 	return func(ctx echo.Context) error {
	// 		if hub := sentryecho.GetHubFromContext(ctx); hub != nil {
	// 			hub.Scope().SetTag("someRandomTag", "maybeYouNeedIt")
	// 		}
	// 		return next(ctx)
	// 	}
	// })

	v1 := e.Group("/v1")

	v1.GET("/serverinfo", apiControllerV1.HandleServerInfo)

	v1.Use(authMiddleware)
	v1.GET("/history", apiControllerV1.HandleGetReceipts)
	v1.POST("/iap/receipt", apiControllerV1.HandleProcessReceipt)
	v1.POST("/braintree/token", apiControllerV1.HandleBraintreeToken)
	v1.POST("/braintree/payment", apiControllerV1.HandleBraintreePayment)
	v1.POST("/paypal/payment", apiControllerV1.HandlePayPalPayment)

	v2 := e.Group("/v2")

	v2iap := v2.Group("/iap")

	v2debug := v2iap.Group("/debug")
	v2debug.POST("/ios", apiControllerV2.DebugReceiptIos)
	v2debug.POST("/android/product", apiControllerV2.DebugReceiptAndroidProduct)
	v2debug.POST("/android/subscription", apiControllerV2.DebugReceiptAndroidSubscription)

	v2iap.Use(authMiddleware)
	v2iap.POST("/ios", apiControllerV2.ProcessReceiptIos)
	v2iap.POST("/android", apiControllerV2.ProcessReceiptAndroid)

	return e
}
