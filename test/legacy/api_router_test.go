package test_test

import (
	"testing"

	"bitbucket.org/calmisland/go-server-api/openapi/openapi3"
	"bitbucket.org/calmisland/go-server-logs/logger"
	"bitbucket.org/calmisland/payment-lambda-funcs/pkg/router"
	test "bitbucket.org/calmisland/payment-lambda-funcs/test/legacy"
)

func TestAPIRouter(t *testing.T) {
	test.Setup()

	api, err := openapi3.Load(apiDefinitionPath)
	if err != nil {
		panic(err)
	}

	backupLogger := logger.GetLogger()
	logger.SetLogger(nil)

	rootRouter := router.InitializeRoutes()
	openapi3.TestRouter(t, api, rootRouter, &openapi3.RouterTestingOptions{
		BasePath:        "/v1/",
		IgnoreResources: []string{},
	})

	logger.SetLogger(backupLogger)
}
