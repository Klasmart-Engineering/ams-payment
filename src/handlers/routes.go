package handlers

import (
	"bitbucket.org/calmisland/go-server-auth/authmiddlewares"
	"bitbucket.org/calmisland/go-server-requests/apirouter"
	"bitbucket.org/calmisland/go-server-requests/standardhandlers"
	"bitbucket.org/calmisland/payment-lambda-funcs/src/globals"
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

	routerV1 := createLambdaRouterV1()
	rootRouter.AddRouter("v1", routerV1)
	return rootRouter
}

func createLambdaRouterV1() *apirouter.Router {
	authMiddleware := authmiddlewares.ValidateSession(globals.AccessTokenValidator, true)

	router := apirouter.NewRouter()
	router.AddMethodHandler("GET", "serverinfo", standardhandlers.HandleServerInfo)

	iapPaymentRouter := apirouter.NewRouter()
	iapPaymentRouter.AddMiddleware(authMiddleware) // Validates the user session
	iapPaymentRouter.AddMethodHandler("POST", "receipt", HandleProcessReceipt)
	router.AddRouter("iap", iapPaymentRouter)

	return router
}
