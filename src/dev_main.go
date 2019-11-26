// +build !lambda

package main

import (
	"bitbucket.org/calmisland/go-server-configs/configs"
	"bitbucket.org/calmisland/go-server-requests/apiservers/httpserver"
	"bitbucket.org/calmisland/payment-lambda-funcs/src/setup/globalsetup"
)

func main() {
	err := configs.UpdateConfigDirectoryPath(configs.DefaultConfigFolderName)
	if err != nil {
		panic(err)
	}
	globalsetup.Setup()
	initLambdaFunctions()
	initLambdaDevFunctions()

	restServer := &httpserver.Server{
		ListenAddress: ":8092",
		Handler:       rootRouter,
	}

	err = restServer.ListenAndServe()
	if err != nil {
		panic(err)
	}
}
