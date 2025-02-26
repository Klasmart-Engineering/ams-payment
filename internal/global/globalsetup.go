package global

import (
	"encoding/base64"
	"fmt"
	"os"

	"github.com/awa/go-iap/playstore"
	log "github.com/sirupsen/logrus"

	"bitbucket.org/calmisland/go-server-account/accountdatabase"
	"bitbucket.org/calmisland/go-server-account/accountdatabase/accountdynamodb"
	"bitbucket.org/calmisland/go-server-aws/awsdynamodb"
	"bitbucket.org/calmisland/go-server-aws/awslambda"
	"bitbucket.org/calmisland/go-server-aws/awssqs"
	"bitbucket.org/calmisland/go-server-configs/configs"
	"bitbucket.org/calmisland/go-server-iap/receiptvalidator/appleappstorereceipts"
	"bitbucket.org/calmisland/go-server-iap/receiptvalidator/googleplaystorereceipts"
	"bitbucket.org/calmisland/go-server-messages/sendmessagequeue"
	"bitbucket.org/calmisland/go-server-product/passaccessservice"
	"bitbucket.org/calmisland/go-server-product/passservice"
	"bitbucket.org/calmisland/go-server-product/productaccessservice"
	"bitbucket.org/calmisland/go-server-product/productdatabase"
	"bitbucket.org/calmisland/go-server-product/productdatabase/productdynamodb"
	"bitbucket.org/calmisland/go-server-product/productservice"
	"bitbucket.org/calmisland/go-server-product/storeproductservice"
	"bitbucket.org/calmisland/go-server-requests/tokens/accesstokens"
	"bitbucket.org/calmisland/go-server-utils/osutils"
	services "bitbucket.org/calmisland/payment-lambda-funcs/internal/services/v1"
	"bitbucket.org/calmisland/payment-lambda-funcs/internal/services/v1/iap"
	services_v2 "bitbucket.org/calmisland/payment-lambda-funcs/internal/services/v2/iap"

	"github.com/getsentry/sentry-go"
)

// Setup setup the server based on configuration
func Setup() {
	// Log as JSON instead of the default ASCII formatter.
	log.SetFormatter(&log.JSONFormatter{})
	log.SetOutput(os.Stdout)

	iap.GetService().Initialize()

	setupSentry()
	SetupSlackMessageService()

	accountDatabase := setupAccountDatabase()
	productDatabase := setupProductDatabase()
	setupServices(accountDatabase, productDatabase)
	setupBraintreePaymentLambda()
	setupPaypalPaymentLambda()

	setupAccessTokenSystems()
	setupGooglePlayClient()
	setupGooglePlayReceiptValidator()
	setupAppleStoreReceiptValidator()

	setupMessageQueue()

	Verify()
}

func setupSentry() {
	if err := sentry.Init(sentry.ClientOptions{}); err != nil {
		fmt.Printf("Sentry initialization failed: %v\n", err)
	}
}

// SetupSlackMessageService setup Slack channel
func SetupSlackMessageService() {
	var config services.SlackConfig

	err := configs.ReadEnvConfig(&config)
	if err != nil {
		panic(err)
	}

	PaymentSlackMessageService = &services.SlackMessageService{WebHookURL: config.PaymentChannel}

}

func setupAccountDatabase() accountdatabase.Database {
	var accountDatabaseConfig awsdynamodb.ClientConfig
	err := configs.ReadEnvConfig(&accountDatabaseConfig)
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
	err := configs.ReadEnvConfig(&productDatabaseConfig)
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

	productService := &productservice.StandardProductService{
		ProductDatabase: productDatabase,
	}

	storeProductService := &storeproductservice.StandardStoreProductService{
		ProductDatabase: productDatabase,
	}

	transactionServiceV2 := &services_v2.TransactionStandardService{
		AccountDatabase:      accountDatabase,
		PassService:          passService,
		PassAccessService:    passAccessService,
		ProductAccessService: productAccessService,
		StoreProductService:  storeProductService,
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
	var validatorConfig accesstokens.ValidatorConfig
	err := configs.ReadEnvConfig(&validatorConfig)
	if err != nil {
		panic(err)
	}

	AccessTokenValidator, err = accesstokens.NewValidator(validatorConfig)
	if err != nil {
		panic(err)
	}
}

func setupGooglePlayClient() {
	jsonKeyBase64 := os.Getenv("GOOGLE_PLAYSTORE_JSON_KEY")
	fmt.Printf("[ENV LOADED]  %s %s\n", "GOOGLE_PLAYSTORE_JSON_KEY", jsonKeyBase64)

	jsonKeyStr, err := base64.StdEncoding.DecodeString(jsonKeyBase64)
	if err != nil {
		panic(err)
	}
	jsonKey := []byte(jsonKeyStr)

	if client, err := playstore.New(jsonKey); err != nil {
		panic(err)
	} else {
		GooglePlayStoreClient = client
	}

}

func setupGooglePlayReceiptValidator() {
	appPublicKeys := iap.GetService().AndroidPublicKeys
	googlePlayReceiptValidator, err := googleplaystorereceipts.NewReceiptValidator(GooglePlayStoreClient, appPublicKeys)

	if err != nil {
		panic(err)
	}

	GooglePlayReceiptValidator = googlePlayReceiptValidator
}

func setupAppleStoreReceiptValidator() {
	if appleAppStoreReceiptValidator, err := appleappstorereceipts.NewReceiptValidator(); err != nil {
		panic(err)
	} else {
		AppleAppStoreReceiptValidator = appleAppStoreReceiptValidator
	}
}

func setupBraintreePaymentLambda() {
	var braintreePaymentLambdaConfig awslambda.FunctionConfig
	braintreePaymentLambdaConfig.Region = osutils.GetOsEnvWithDef("AMS_AWS_FUNCTION_BRAINTREE_PAYMENT_REGION", "")
	fmt.Printf("[ENV LOADED]  %s %s\n", "AMS_AWS_FUNCTION_BRAINTREE_PAYMENT_REGION", braintreePaymentLambdaConfig.Region)
	braintreePaymentLambdaConfig.FunctionName = osutils.GetOsEnvWithDef("AMS_AWS_FUNCTION_BRAINTREE_PAYMENT_NAME", "")
	fmt.Printf("[ENV LOADED]  %s %s\n", "AMS_AWS_FUNCTION_BRAINTREE_PAYMENT_NAME", braintreePaymentLambdaConfig.FunctionName)

	var err error
	BraintreePaymentFunction, err = awslambda.NewFunction(&braintreePaymentLambdaConfig)
	if err != nil {
		panic(err)
	}
}

func setupPaypalPaymentLambda() {
	var paypalPaymentLambdaConfig awslambda.FunctionConfig
	paypalPaymentLambdaConfig.Region = osutils.GetOsEnvWithDef("AMS_AWS_FUNCTION_PAYPAL_PAYMENT_REGION", "")
	fmt.Printf("[ENV LOADED]  %s %s\n", "AMS_AWS_FUNCTION_PAYPAL_PAYMENT_REGION", paypalPaymentLambdaConfig.Region)
	paypalPaymentLambdaConfig.FunctionName = osutils.GetOsEnvWithDef("AMS_AWS_FUNCTION_PAYPAL_PAYMENT_NAME", "")
	fmt.Printf("[ENV LOADED]  %s %s\n", "AMS_AWS_FUNCTION_PAYPAL_PAYMENT_NAME", paypalPaymentLambdaConfig.FunctionName)

	var err error
	PayPalPaymentFunction, err = awslambda.NewFunction(&paypalPaymentLambdaConfig)
	if err != nil {
		panic(err)
	}
}

func setupMessageQueue() {
	var queueConfig awssqs.QueueConfig
	err := configs.ReadEnvConfig(&queueConfig)
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
