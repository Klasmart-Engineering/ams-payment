// +build !lambda

package main

import (
	"context"

	"bitbucket.org/calmisland/go-server-account/accountdatabase"

	"bitbucket.org/calmisland/go-server-requests/apirequests"
	"bitbucket.org/calmisland/go-server-requests/apirouter"
)

func initLambdaDevFunctions() {
	devRouter := apirouter.NewRouter()
	devRouter.AddMethodHandler("GET", "createtables", createTablesRequest)
	rootRouter.AddRouter("dev", devRouter)
}

func createTablesRequest(ctx context.Context, req *apirequests.Request, resp *apirequests.Response) error {
	// Get the database
	db, err := accountdatabase.GetDatabase()
	if err != nil {
		return resp.SetServerError(err)
	}

	err = db.CreateDatabaseTables()
	if err != nil {
		return resp.SetServerError(err)
	}

	resp.SetStatus(200)

	return nil
}
