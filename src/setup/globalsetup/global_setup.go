package globalsetup

import (
	"bitbucket.org/calmisland/go-server-account/accountdatabase"
	"bitbucket.org/calmisland/go-server-account/accountdatabase/accountdynamodb"
	"bitbucket.org/calmisland/go-server-aws/awsdynamodb"
	"bitbucket.org/calmisland/go-server-configs/configs"
	"bitbucket.org/calmisland/go-server-iap/receiptvalidator/appleappstorereceipts"
	"bitbucket.org/calmisland/go-server-iap/receiptvalidator/googleplaystorereceipts"
	"bitbucket.org/calmisland/go-server-logs/errorreporter"
	"bitbucket.org/calmisland/go-server-logs/errorreporter/slackreporter"
	"bitbucket.org/calmisland/go-server-product/passaccessservice"
	"bitbucket.org/calmisland/go-server-product/passservice"
	"bitbucket.org/calmisland/go-server-product/productaccessservice"
	"bitbucket.org/calmisland/go-server-product/productdatabase"
	"bitbucket.org/calmisland/go-server-product/productdatabase/productdynamodb"
	"bitbucket.org/calmisland/go-server-product/productservice"
	"bitbucket.org/calmisland/go-server-product/storeproductservice"
	"bitbucket.org/calmisland/go-server-requests/tokens/accesstokens"
	"bitbucket.org/calmisland/payment-lambda-funcs/src/globals"
	"bitbucket.org/calmisland/payment-lambda-funcs/src/services"
)

// Setup setup the server based on configuration
func Setup() {
	setupSlackReporter()

	accountDatabase := setupAccountDatabase()
	productDatabase := setupProductDatabase()
	setupServices(accountDatabase, productDatabase)

	setupAccessTokenSystems()
	setupGooglePlayReceiptValidator()
	setupAppleStoreReceiptValidator()

	globals.Verify()
}

func setupAccountDatabase() accountdatabase.Database {
	var accountDatabaseConfig awsdynamodb.ClientConfig
	err := configs.LoadConfig("account_database_dynamodb", &accountDatabaseConfig, true)
	if err != nil {
		panic(err)
	}

	ddbClient, err := awsdynamodb.NewClient(&accountDatabaseConfig)
	if err != nil {
		panic(err)
	}

	accountDatabase, err := accountdynamodb.New(ddbClient)
	if err != nil {
		panic(err)
	}

	globals.AccountDatabase = accountDatabase
	return accountDatabase
}

func setupProductDatabase() *productdynamodb.Database {
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

	globals.ProductDatabase = productDatabase
	return productDatabase
}

func setupServices(accountDatabase accountdatabase.Database, productDatabase productdatabase.Database) {
	productAccessService := &productaccessservice.StandardProductAccessService{
		ProductDatabase: productDatabase,
	}
	passAccessService := &passaccessservice.StandardPassAccessService{
		ProductDatabase: productDatabase,
	}

	transactionService := &services.TransactionStandardService{
		AccountDatabase:      accountDatabase,
		PassAccessService:    passAccessService,
		ProductAccessService: productAccessService,
	}

	passService := &passservice.StandardPassService{
		ProductDatabase: productDatabase,
	}

	productService := &productservice.StandardProductService{
		ProductDatabase: productDatabase,
	}

	storeProductService := &storeproductservice.StandardStoreProductService{
		ProductDatabase: productDatabase,
	}

	globals.ProductAccessService = productAccessService
	globals.PassAccessService = passAccessService
	globals.TransactionService = transactionService
	globals.PassService = passService
	globals.ProductService = productService
	globals.StoreProductService = storeProductService
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

func setupGooglePlayReceiptValidator() {
	var googlePlayValidatorConfig googleplaystorereceipts.ReceiptValidatorConfig
	err := configs.LoadConfig("googleplay_receipt_validator", &googlePlayValidatorConfig, false)
	if err != nil {
		panic(err)
	}

	if len(googlePlayValidatorConfig.JSONKey) > 0 || len(googlePlayValidatorConfig.JSONKeyFile) > 0 {
		globals.GooglePlayReceiptValidator, err = googleplaystorereceipts.NewReceiptValidator(googlePlayValidatorConfig)
		if err != nil {
			panic(err)
		}
	} else {
		globals.GooglePlayReceiptValidator = nil
	}
}

func setupAppleStoreReceiptValidator() {
	var appleStoreValidatorConfig appleappstorereceipts.ReceiptValidatorConfig
	err := configs.LoadConfig("applestore_receipt_validator", &appleStoreValidatorConfig, false)
	if err != nil {
		panic(err)
	}

	if len(appleStoreValidatorConfig.Password) > 0 {
		globals.AppleAppStoreReceiptValidator, err = appleappstorereceipts.NewReceiptValidator(appleStoreValidatorConfig)
		if err != nil {
			panic(err)
		}
	} else {
		globals.AppleAppStoreReceiptValidator = nil
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
