// +build lambda

package main

import (
	"bitbucket.org/calmisland/go-server-aws/awslambda"
	"bitbucket.org/calmisland/go-server-configs/configs"
	"bitbucket.org/calmisland/payment-lambda-funcs/src/server"
)

func main() {
	err := configs.UpdateConfigDirectoryPath(configs.DefaultConfigFolderName)
	if err != nil {
		panic(err)
	}

	server.Setup()
	initLambdaFunctions()

	err = awslambda.StartAPIHandler(rootRouter)
	if err != nil {
		panic(err)
	}
}