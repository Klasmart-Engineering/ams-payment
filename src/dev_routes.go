// +build !lambda

package main

import (
	"context"

	"bitbucket.org/calmisland/go-server-auth/authmiddlewares"
	"bitbucket.org/calmisland/go-server-requests/apirequests"
	"bitbucket.org/calmisland/go-server-requests/apirouter"
	"bitbucket.org/calmisland/payment-lambda-funcs/src/globals"
)

func initLambdaDevFunctions(rootRouter *apirouter.Router) {
	authMiddleware := authmiddlewares.ValidateSession(globals.AccessTokenValidator, true)
	devRouter := apirouter.NewRouter()
	devRouter.AddMiddleware(authMiddleware) // Validates the user session

	devRouter.AddMethodHandler("GET", "createtables", createTablesRequest)

	rootRouter.AddRouter("dev", devRouter)
}

func createTablesRequest(ctx context.Context, req *apirequests.Request, resp *apirequests.Response) error {
	err := globals.AccountDatabase.CreateDatabaseTables()
	if err != nil {
		return resp.SetServerError(err)
	}

	resp.SetStatus(200)

	return nil
}
