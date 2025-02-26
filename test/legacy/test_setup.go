package test

import (
	"bitbucket.org/calmisland/go-server-account/accountdatabase"
	"bitbucket.org/calmisland/go-server-account/accountdatabase/accountmemorydb"
	"bitbucket.org/calmisland/go-server-cloud/cloudfunctions"
	"bitbucket.org/calmisland/go-server-cloud/cloudfunctions/cloudfunctionsmock"
	"bitbucket.org/calmisland/go-server-messages/sendmessagequeue/sendmessagequeuemock"
	"bitbucket.org/calmisland/go-server-product/passaccessservice"
	"bitbucket.org/calmisland/go-server-product/passservice"
	"bitbucket.org/calmisland/go-server-product/productaccessservice"
	"bitbucket.org/calmisland/go-server-product/productdatabase"
	"bitbucket.org/calmisland/go-server-product/productdatabase/productmemorydb"
	"bitbucket.org/calmisland/go-server-product/productservice"
	"bitbucket.org/calmisland/go-server-product/storeproductservice"
	"bitbucket.org/calmisland/go-server-requests/sessions"
	"bitbucket.org/calmisland/go-server-requests/tokens/accesstokens/accesstokensmock"
	"bitbucket.org/calmisland/payment-lambda-funcs/internal/global"
	services "bitbucket.org/calmisland/payment-lambda-funcs/internal/services/v1"
	"github.com/calmisland/go-testify/mock"
)

// Setup setup the server based on configuration
func Setup() {
	accountDatabase := setupAccountDatabase()
	productDatabase := setupProductDatabase()
	setupServices(accountDatabase, productDatabase)
	setupBraintreePaymentLambda()
	setupPaypalPaymentLambda()

	setupAccessTokenSystems()
	setupGooglePlayReceiptValidator()
	setupAppleStoreReceiptValidator()

	setupMessageQueue()

	global.Verify()
}

func setupAccountDatabase() accountdatabase.Database {
	accountDatabase := accountmemorydb.New()
	global.AccountDatabase = accountDatabase
	return accountDatabase
}

func setupProductDatabase() productdatabase.Database {
	productDatabase := productmemorydb.New()
	global.ProductDatabase = productDatabase
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

	global.ProductAccessService = productAccessService
	global.PassAccessService = passAccessService
	global.TransactionService = transactionService
	global.PassService = passService
	global.ProductService = productService
	global.StoreProductService = storeProductService
}

func setupAccessTokenSystems() {
	accessTokenValidator := &accesstokensmock.MockValidator{}
	accessTokenValidator.On("ValidateAccessToken", mock.Anything).Return(&sessions.SessionData{
		SessionID: "TEST-SESSION",
		AccountID: "TEST-ACCOUNT",
		DeviceID:  "TEST-DEVICE",
	}, nil)

	global.AccessTokenValidator = accessTokenValidator
}

func setupGooglePlayReceiptValidator() {
	global.GooglePlayReceiptValidator = nil
}

func setupAppleStoreReceiptValidator() {
	global.AppleAppStoreReceiptValidator = nil
}

func setupBraintreePaymentLambda() {
	mockfn := cloudfunctionsmock.NewMockFunction()
	mockfn.On("Invoke", mock.Anything).Return(&cloudfunctions.FunctionInvokeOutput{
		Payload: []byte("{}"),
		IsError: false,
	}, nil)
	global.BraintreePaymentFunction = mockfn
}

func setupPaypalPaymentLambda() {
	mockfn := cloudfunctionsmock.NewMockFunction()
	mockfn.On("Invoke", mock.Anything).Return(&cloudfunctions.FunctionInvokeOutput{
		Payload: []byte("{}"),
		IsError: false,
	}, nil)
	global.PayPalPaymentFunction = mockfn
}

func setupMessageQueue() {
	messageSendQueue := &sendmessagequeuemock.QueueMock{}
	messageSendQueue.On("EnqueueMessage", mock.Anything).Return(nil)
	global.MessageSendQueue = messageSendQueue
}
