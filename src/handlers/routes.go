package handlers

import (
	"bitbucket.org/calmisland/go-server-auth/authmiddlewares"
	"bitbucket.org/calmisland/go-server-requests/apirouter"
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
	routerV1 := createLambdaRouterV1()
	rootRouter.AddRouter("v1", routerV1)
	return rootRouter
}

func createLambdaRouterV1() *apirouter.Router {
	authMiddleware := authmiddlewares.ValidateSession(globals.AccessTokenValidator, true)

	router := apirouter.NewRouter()
	router.AddMiddleware(authMiddleware) // Validates the user session

	router.AddMethodHandler("GET", "serverinfo", HandleServerInfo)

	return router
}
