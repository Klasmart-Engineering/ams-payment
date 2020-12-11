package main

import (
	"context"
	"fmt"

	"bitbucket.org/calmisland/go-server-auth/authmiddlewares"
	"bitbucket.org/calmisland/go-server-configs/configs"
	"bitbucket.org/calmisland/go-server-requests/apirequests"
	"bitbucket.org/calmisland/go-server-requests/apirouter"
	"bitbucket.org/calmisland/go-server-requests/apiservers/httpserver"
	"bitbucket.org/calmisland/payment-lambda-funcs/pkg/global"
	"bitbucket.org/calmisland/payment-lambda-funcs/pkg/router"
)

func main() {
	fmt.Println("hello world")
	err := configs.UpdateConfigDirectoryPath(configs.DefaultConfigFolderName)
	if err != nil {
		panic(err)
	}

	global.Setup()
	rootRouter := router.InitializeRoutes()
	initLambdaDevFunctions(rootRouter)

	restServer := &httpserver.Server{
		ListenAddress: ":8092",
		Handler:       rootRouter,
	}

	println("started - port: 8092")
	err = restServer.ListenAndServe()
	if err != nil {
		panic(err)
	}
}

func initLambdaDevFunctions(rootRouter *apirouter.Router) {
	authMiddleware := authmiddlewares.ValidateSession(global.AccessTokenValidator, true)
	devRouter := apirouter.NewRouter()
	devRouter.AddMiddleware(authMiddleware) // Validates the user session

	devRouter.AddMethodHandler("GET", "createtables", createTablesRequest)

	rootRouter.AddRouter("dev", devRouter)
}

func createTablesRequest(ctx context.Context, req *apirequests.Request, resp *apirequests.Response) error {
	err := global.AccountDatabase.CreateDatabaseTables()
	if err != nil {
		return resp.SetServerError(err)
	}

	resp.SetStatus(200)

	return nil
}
