package test_test

import (
	"testing"

	"bitbucket.org/calmisland/payment-lambda-funcs/pkg/handler"
	"bitbucket.org/calmisland/payment-lambda-funcs/test/legacy"
	"bitbucket.org/calmisland/go-server-api/openapi/openapi3"
	"bitbucket.org/calmisland/go-server-logs/logger"
)

func TestAPIRouter(t *testing.T) {
	testsetup.Setup()

	api, err := openapi3.Load(apiDefinitionPath)
	if err != nil {
		panic(err)
	}

	backupLogger := logger.GetLogger()
	logger.SetLogger(nil)

	rootRouter := handlers.InitializeRoutes()
	openapi3.TestRouter(t, api, rootRouter, &openapi3.RouterTestingOptions{
		BasePath:        "/v1/",
		IgnoreResources: []string{},
	})

	logger.SetLogger(backupLogger)
}
