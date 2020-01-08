package testsetup

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
	"bitbucket.org/calmisland/payment-lambda-funcs/src/globals"
	"bitbucket.org/calmisland/payment-lambda-funcs/src/services"
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

	globals.Verify()
}

func setupAccountDatabase() accountdatabase.Database {
	accountDatabase := accountmemorydb.New()
	globals.AccountDatabase = accountDatabase
	return accountDatabase
}

func setupProductDatabase() productdatabase.Database {
	productDatabase := productmemorydb.New()
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
	accessTokenValidator := &accesstokensmock.MockValidator{}
	accessTokenValidator.On("ValidateAccessToken", mock.Anything).Return(&sessions.SessionData{
		SessionID: "TEST-SESSION",
		AccountID: "TEST-ACCOUNT",
		DeviceID:  "TEST-DEVICE",
	}, nil)

	globals.AccessTokenValidator = accessTokenValidator
}

func setupGooglePlayReceiptValidator() {
	globals.GooglePlayReceiptValidator = nil
}

func setupAppleStoreReceiptValidator() {
	globals.AppleAppStoreReceiptValidator = nil
}

func setupBraintreePaymentLambda() {
	mockfn := cloudfunctionsmock.NewMockFunction()
	mockfn.On("Invoke", mock.Anything).Return(&cloudfunctions.FunctionInvokeOutput{
		Payload: []byte("{}"),
		IsError: false,
	}, nil)
	globals.BraintreePaymentFunction = mockfn
}

func setupPaypalPaymentLambda() {
	mockfn := cloudfunctionsmock.NewMockFunction()
	mockfn.On("Invoke", mock.Anything).Return(&cloudfunctions.FunctionInvokeOutput{
		Payload: []byte("{}"),
		IsError: false,
	}, nil)
	globals.PayPalPaymentFunction = mockfn
}

func setupMessageQueue() {
	messageSendQueue := &sendmessagequeuemock.QueueMock{}
	messageSendQueue.On("EnqueueMessage", mock.Anything).Return(nil)
	globals.MessageSendQueue = messageSendQueue
}
