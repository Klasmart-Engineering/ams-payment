package main

import (
	"bitbucket.org/calmisland/go-server-auth/authmiddlewares"
	"bitbucket.org/calmisland/go-server-requests/apirouter"
	"bitbucket.org/calmisland/payment-lambda-funcs/src/globals"
	"bitbucket.org/calmisland/payment-lambda-funcs/src/handlers"
)

var (
	rootRouter *apirouter.Router
)

func initLambdaFunctions() {
	rootRouter = apirouter.NewRouter()
	routerV1 := createLambdaRouterV1()
	rootRouter.AddRouter("v1", routerV1)
}

func createLambdaRouterV1() *apirouter.Router {
	authMiddleware := authmiddlewares.ValidateSession(globals.AccessTokenValidator, true)

	router := apirouter.NewRouter()
	router.AddMiddleware(authMiddleware) // Validates the user session

	router.AddMethodHandler("GET", "serverinfo", handlers.HandleServerInfo)

	return router
}
