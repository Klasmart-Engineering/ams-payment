package routes

import (
	"bitbucket.org/calmisland/go-server-auth/authmiddlewares"
	"bitbucket.org/calmisland/go-server-requests/apirouter"
	"bitbucket.org/calmisland/go-server-requests/standardhandlers"
	"bitbucket.org/calmisland/payment-lambda-funcs/src/globals"
	"bitbucket.org/calmisland/payment-lambda-funcs/src/handlers"
	"bitbucket.org/calmisland/payment-lambda-funcs/src/handlers2"
)

var (
	rootRouter *apirouter.Router
)

// InitializeRoutes initializes the routes.
func InitializeRoutes() *apirouter.Router {
	if rootRouter != nil {
		return rootRouter
	}

	rootRouter = apirouter.NewRouter()
	if globals.CORSOptions != nil {
		rootRouter.AddCORSMiddleware(globals.CORSOptions)
	}

	routerV1 := createRouterV1()
	rootRouter.AddRouter("v1", routerV1)

	routerV2 := createRouterV2()
	rootRouter.AddRouter("v2", routerV2)

	return rootRouter
}

func createRouterV1() *apirouter.Router {
	authMiddleware := authmiddlewares.ValidateSession(globals.AccessTokenValidator, true)

	router := apirouter.NewRouter()
	router.AddMethodHandler("GET", "serverinfo", standardhandlers.HandleServerInfo)

	// router.AddMiddleware(authMiddleware) // Validates the user session

	router.AddMethodHandler("GET", "history", handlers.HandleGetReceipts, authMiddleware)

	iapPaymentRouter := apirouter.NewRouter()
	iapPaymentRouter.AddMiddleware(authMiddleware) // Validates the user session
	iapPaymentRouter.AddMethodHandler("POST", "receipt", handlers.HandleProcessReceipt)
	router.AddRouter("iap", iapPaymentRouter)

	braintreeRouter := apirouter.NewRouter()
	braintreeRouter.AddMiddleware(authMiddleware) // Validates the user session
	braintreeRouter.AddMethodHandler("GET", "token", handlers.HandleBraintreeToken)
	braintreeRouter.AddMethodHandler("POST", "payment", handlers.HandleBraintreePayment)
	router.AddRouter("braintree", braintreeRouter)

	paypalRouter := apirouter.NewRouter()
	paypalRouter.AddMiddleware(authMiddleware) // Validates the user session
	paypalRouter.AddMethodHandler("POST", "payment", handlers.HandlePayPalPayment)
	router.AddRouter("paypal", paypalRouter)

	return router
}

func createRouterV2() *apirouter.Router {
	authMiddleware := authmiddlewares.ValidateSession(globals.AccessTokenValidator, true)

	router := apirouter.NewRouter()
	router.AddMiddleware(authMiddleware) // Validates the user session

	iapPaymentRouter := apirouter.NewRouter()

	iapPaymentRouter.AddMethodHandler("POST", "android", handlers2.ProcessReceiptAndroid)
	iapPaymentRouter.AddMethodHandler("POST", "ios", handlers2.ProcessReceiptIos)

	debugRouter := apirouter.NewRouter()

	debugRouter.AddMethodHandler("POST", "android", handlers2.DebugReceiptAndroid)
	debugRouter.AddMethodHandler("POST", "ios", handlers2.DebugReceiptIos)

	iapPaymentRouter.AddRouter("debug", debugRouter)

	router.AddRouter("iap", iapPaymentRouter)

	return router
}
