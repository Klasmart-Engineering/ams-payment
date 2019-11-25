// +build !china

package server

import (
	"bitbucket.org/calmisland/go-server-account/accountdatabase/accountdynamodb"
	"bitbucket.org/calmisland/go-server-aws/awsdynamodb"
	"bitbucket.org/calmisland/go-server-configs/configs"
	"bitbucket.org/calmisland/go-server-logs/errorreporter"
	"bitbucket.org/calmisland/go-server-logs/errorreporter/slackreporter"
	"bitbucket.org/calmisland/go-server-product/passaccessservice"
	"bitbucket.org/calmisland/go-server-product/productaccessservice"
	"bitbucket.org/calmisland/go-server-product/productdatabase/productdynamodb"
	"bitbucket.org/calmisland/go-server-requests/tokens/accesstokens"
	"bitbucket.org/calmisland/payment-lambda-funcs/src/globals"
)

// Setup setup the server based on configuration
func Setup() {
	setupAccountDatabase()
	setupProductAndPassAccessService()

	setupAccessTokenSystems()
	setupSlackReporter()
}

func setupAccountDatabase() {
	var accountDatabaseConfig awsdynamodb.ClientConfig
	err := configs.LoadConfig("account_database_dynamodb", &accountDatabaseConfig, true)
	if err != nil {
		panic(err)
	}

	ddbClient, err := awsdynamodb.NewClient(&accountDatabaseConfig)
	if err != nil {
		panic(err)
	}

	globals.AccountDatabase, err = accountdynamodb.New(ddbClient)
	if err != nil {
		panic(err)
	}
}

func setupProductAndPassAccessService() {
	var productDatabaseConfig awsdynamodb.ClientConfig
	err := configs.LoadConfig("product_database_dynamodb", &productDatabaseConfig, true)
	if err != nil {
		panic(err)
	}

	ddbClient, err := awsdynamodb.NewClient(&productDatabaseConfig)
	if err != nil {
		panic(err)
	}

	productDatabase, err := productdynamodb.New(ddbClient)
	if err != nil {
		panic(err)
	}

	globals.ProductAccessService = &productaccessservice.StandardProductAccessService{
		ProductDatabase: productDatabase,
	}
	globals.PassAccessService = &passaccessservice.StandardPassAccessService{
		ProductDatabase: productDatabase,
	}
}

func setupAccessTokenSystems() {
	var validatorConfig accesstokens.ValidatorConfig
	err := configs.LoadConfig("access_tokens", &validatorConfig, true)
	if err != nil {
		panic(err)
	}

	globals.AccessTokenValidator, err = accesstokens.NewValidator(validatorConfig)
	if err != nil {
		panic(err)
	}
}

func setupSlackReporter() {
	var slackReporterConfig slackreporter.Config
	err := configs.LoadConfig("error_reporter_slack", &slackReporterConfig, false)
	if err != nil {
		panic(err)
	}

	// Check if there is a configuration for the Slack error reporter
	if len(slackReporterConfig.HookURL) == 0 {
		return
	}

	reporter, err := slackreporter.New(&slackReporterConfig)
	if err != nil {
		panic(err)
	}

	errorreporter.Active = reporter
}
