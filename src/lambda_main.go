// +build lambda

package main

import (
	"bitbucket.org/calmisland/go-server-aws/awslambda"
	"bitbucket.org/calmisland/go-server-configs/configs"
	"bitbucket.org/calmisland/payment-lambda-funcs/src/handlers"
	"bitbucket.org/calmisland/payment-lambda-funcs/src/setup/globalsetup"
)

func main() {
	err := configs.UpdateConfigDirectoryPath(configs.DefaultConfigFolderName)
	if err != nil {
		panic(err)
	}

	globalsetup.Setup()
	rootRouter := handlers.InitializeRoutes()

	err = awslambda.StartAPIHandler(rootRouter)
	if err != nil {
		panic(err)
	}
}
