// +build lambda

package main

import (
	"bitbucket.org/calmisland/go-server-aws/awslambda"
	"bitbucket.org/calmisland/go-server-configs/configs"
	"bitbucket.org/calmisland/payment-lambda-funcs/pkg/handler"
	"bitbucket.org/calmisland/payment-lambda-funcs/pkg/global"
)

func main() {
	err := configs.UpdateConfigDirectoryPath(configs.DefaultConfigFolderName)
	if err != nil {
		panic(err)
	}

	global.Setup()
	rootRouter := routes.InitializeRoutes()

	err = awslambda.StartAPIHandler(rootRouter)
	if err != nil {
		panic(err)
	}
}
