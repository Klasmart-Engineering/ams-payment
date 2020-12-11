package global

import (
	"errors"
	"fmt"
	"os"

	log "github.com/sirupsen/logrus"

	"bitbucket.org/calmisland/go-server-account/accountdatabase"
	"bitbucket.org/calmisland/go-server-account/accountdatabase/accountdynamodb"
	"bitbucket.org/calmisland/go-server-aws/awsdynamodb"
	"bitbucket.org/calmisland/go-server-aws/awslambda"
	"bitbucket.org/calmisland/go-server-aws/awssqs"
	"bitbucket.org/calmisland/go-server-configs/configs"
	"bitbucket.org/calmisland/go-server-iap/receiptvalidator/appleappstorereceipts"
	"bitbucket.org/calmisland/go-server-iap/receiptvalidator/googleplaystorereceipts"
	"bitbucket.org/calmisland/go-server-logs/errorreporter"
	"bitbucket.org/calmisland/go-server-logs/errorreporter/slackreporter"
	"bitbucket.org/calmisland/go-server-messages/sendmessagequeue"
	"bitbucket.org/calmisland/go-server-product/passaccessservice"
	"bitbucket.org/calmisland/go-server-product/passservice"
	"bitbucket.org/calmisland/go-server-product/productaccessservice"
	"bitbucket.org/calmisland/go-server-product/productdatabase"
	"bitbucket.org/calmisland/go-server-product/productdatabase/productdynamodb"
	"bitbucket.org/calmisland/go-server-product/productservice"
	"bitbucket.org/calmisland/go-server-product/storeproductservice"
	"bitbucket.org/calmisland/go-server-requests/apirouter"
	"bitbucket.org/calmisland/go-server-requests/tokens/accesstokens"
	"bitbucket.org/calmisland/payment-lambda-funcs/pkg/iap"
	"bitbucket.org/calmisland/payment-lambda-funcs/pkg/service"
	services_v2 "bitbucket.org/calmisland/payment-lambda-funcs/pkg/service2"

	"github.com/getsentry/sentry-go"
)

// Setup setup the server based on configuration
func Setup() {
	// Log as JSON instead of the default ASCII formatter.
	log.SetFormatter(&log.JSONFormatter{})
	log.SetOutput(os.Stdout)

	iap.GetService().Initialize()
	setupSentry()
	setupSlackReporter()
	SetupSlackMessageService()

	accountDatabase := setupAccountDatabase()
	productDatabase := setupProductDatabase()
	setupServices(accountDatabase, productDatabase)
	setupBraintreePaymentLambda()
	setupPaypalPaymentLambda()

	setupAccessTokenSystems()
	setupGooglePlayReceiptValidator()
	setupAppleStoreReceiptValidator()
	setupCORS()

	setupMessageQueue()

	Verify()
}

func setupSentry() {
	var env string = fmt.Sprintf("%s@%s", os.Getenv("SERVER_STAGE"), os.Getenv("SERVER_REGION"))
	fmt.Println(env)
	err := sentry.Init(sentry.ClientOptions{
		Dsn:         "https://f8d1fc600ed24b4581f7d2d5ea37aecb@o412774.ingest.sentry.io/5413073",
		Environment: env,
	})
	if err != nil {
		log.Fatalf("sentry.Init: %s", err)
	}
}

// SetupSlackMessageService setup Slack channel
func SetupSlackMessageService() {
	var config services.SlackConfig

	err := configs.LoadConfig("slack", &config, true)
	if err != nil {
		panic(err)
	}

	PaymentSlackMessageService = &services.SlackMessageService{WebHookURL: config.PaymentChannel}

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

	AccountDatabase = accountDatabase
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

	ProductDatabase = productDatabase
	return productDatabase
}

func setupServices(accountDatabase accountdatabase.Database, productDatabase productdatabase.Database) {
	productAccessService := &productaccessservice.StandardProductAccessService{
		ProductDatabase: productDatabase,
	}
	passAccessService := &passaccessservice.StandardPassAccessService{
		ProductDatabase: productDatabase,
	}

	passService := &passservice.StandardPassService{
		ProductDatabase: productDatabase,
	}

	transactionService := &services.TransactionStandardService{
		AccountDatabase:      accountDatabase,
		PassService:          passService,
		PassAccessService:    passAccessService,
		ProductAccessService: productAccessService,
	}

	transactionServiceV2 := &services_v2.TransactionStandardService{
		AccountDatabase:      accountDatabase,
		PassService:          passService,
		PassAccessService:    passAccessService,
		ProductAccessService: productAccessService,
	}

	productService := &productservice.StandardProductService{
		ProductDatabase: productDatabase,
	}

	storeProductService := &storeproductservice.StandardStoreProductService{
		ProductDatabase: productDatabase,
	}

	ProductAccessService = productAccessService
	PassAccessService = passAccessService
	TransactionService = transactionService
	TransactionServiceV2 = transactionServiceV2
	PassService = passService
	ProductService = productService
	StoreProductService = storeProductService
}

func setupAccessTokenSystems() {
	var err error
	var validatorConfig accesstokens.ValidatorConfig

	bPublicKey := configs.LoadBinary("account.pub")
	if bPublicKey == nil {
		panic(errors.New("the account.pub file is mandatory"))
	}

	validatorConfig.PublicKey = string(bPublicKey)

	AccessTokenValidator, err = accesstokens.NewValidator(validatorConfig)
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

	googlePlayValidatorConfig.AppPublicKeys = iap.GetService().AndroidPublicKeys

	if len(googlePlayValidatorConfig.JSONKey) > 0 || len(googlePlayValidatorConfig.JSONKeyFile) > 0 {
		GooglePlayReceiptValidator, err = googleplaystorereceipts.NewReceiptValidator(googlePlayValidatorConfig)
		if err != nil {
			panic(err)
		}
	} else {
		GooglePlayReceiptValidator = nil
		panic("Failed to generate Google Play Receipt validator")
	}

}

func setupAppleStoreReceiptValidator() {
	var appleStoreValidatorConfig appleappstorereceipts.ReceiptValidatorConfig
	err := configs.LoadConfig("applestore_receipt_validator", &appleStoreValidatorConfig, false)
	if err != nil {
		panic(err)
	}

	if len(appleStoreValidatorConfig.Password) > 0 {
		AppleAppStoreReceiptValidator, err = appleappstorereceipts.NewReceiptValidator(appleStoreValidatorConfig)
		if err != nil {
			panic(err)
		}
	} else {
		AppleAppStoreReceiptValidator = nil
	}
}

func setupCORS() {
	var corsConfig apirouter.CORSOptions
	err := configs.LoadConfig("cross_origin_resource_sharing", &corsConfig, true)
	if err != nil {
		panic(err)
	}

	CORSOptions = &corsConfig
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

func setupBraintreePaymentLambda() {
	var braintreePaymentLambdaConfig awslambda.FunctionConfig
	var err error
	err = configs.LoadConfig("braintree_payment_func_lambda", &braintreePaymentLambdaConfig, true)
	if err != nil {
		panic(err)
	}

	BraintreePaymentFunction, err = awslambda.NewFunction(&braintreePaymentLambdaConfig)
	if err != nil {
		panic(err)
	}
}

func setupPaypalPaymentLambda() {
	var paypalPaymentLambdaConfig awslambda.FunctionConfig
	var err error
	err = configs.LoadConfig("paypal_payment_func_lambda", &paypalPaymentLambdaConfig, true)
	if err != nil {
		panic(err)
	}

	PayPalPaymentFunction, err = awslambda.NewFunction(&paypalPaymentLambdaConfig)
	if err != nil {
		panic(err)
	}
}

func setupMessageQueue() {
	var queueConfig awssqs.QueueConfig
	err := configs.LoadConfig("message_send_sqs", &queueConfig, true)
	if err != nil {
		panic(err)
	}

	messageSendQueue, err := awssqs.NewQueue(queueConfig)
	if err != nil {
		panic(err)
	}

	MessageSendQueue, err = sendmessagequeue.New(sendmessagequeue.QueueConfig{
		Queue: messageSendQueue,
	})
	if err != nil {
		panic(err)
	}
}