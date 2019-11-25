// +build !lambda

package main

import (
	"context"

	"bitbucket.org/calmisland/go-server-requests/apirequests"
	"bitbucket.org/calmisland/go-server-requests/apirouter"
	"bitbucket.org/calmisland/payment-lambda-funcs/src/globals"
)

func initLambdaDevFunctions() {
	devRouter := apirouter.NewRouter()
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
