package testsetup

import (
	"bitbucket.org/calmisland/go-server-account/accountdatabase"
	"bitbucket.org/calmisland/go-server-account/accountdatabase/accountmemorydb"
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

	setupAccessTokenSystems()
	setupGooglePlayReceiptValidator()
	setupAppleStoreReceiptValidator()

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
