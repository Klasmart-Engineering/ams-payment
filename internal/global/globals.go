package global

import (
	"bitbucket.org/calmisland/go-server-account/accountdatabase"
	"bitbucket.org/calmisland/go-server-cloud/cloudfunctions"
	"bitbucket.org/calmisland/go-server-iap/receiptvalidator"
	"bitbucket.org/calmisland/go-server-messages/sendmessagequeue"
	"bitbucket.org/calmisland/go-server-product/passaccessservice"
	"bitbucket.org/calmisland/go-server-product/passservice"
	"bitbucket.org/calmisland/go-server-product/productaccessservice"
	"bitbucket.org/calmisland/go-server-product/productdatabase"
	"bitbucket.org/calmisland/go-server-product/productservice"
	"bitbucket.org/calmisland/go-server-product/storeproductservice"
	"bitbucket.org/calmisland/go-server-requests/tokens/accesstokens"
	services "bitbucket.org/calmisland/payment-lambda-funcs/internal/services/v1"
	services_v2 "bitbucket.org/calmisland/payment-lambda-funcs/internal/services/v2/iap"
	"github.com/awa/go-iap/playstore"
	"github.com/calmisland/go-errors"
)

var (
	// AccessTokenValidator is the access token validator
	AccessTokenValidator accesstokens.Validator
	// AccountDatabase is the account database.
	AccountDatabase accountdatabase.Database
	// ProductDatabase is the product database.
	ProductDatabase productdatabase.Database
	// TransactionService aids with payments processing
	TransactionService *services.TransactionStandardService
	// TransactionServiceV2 aids with payments processing for v2
	TransactionServiceV2 *services_v2.TransactionStandardService
	// ProductAccessService allows use of the product database
	ProductAccessService *productaccessservice.StandardProductAccessService
	// PassAccessService allows use of the product database
	PassAccessService *passaccessservice.StandardPassAccessService
	// PassService allows use of the product database
	PassService *passservice.StandardPassService
	// ProductService allows use of the product database
	ProductService *productservice.StandardProductService
	// StoreProductService allows use of the store database
	StoreProductService *storeproductservice.StandardStoreProductService
	// GooglePlayStoreClient is the googleplay client for receipt validation
	GooglePlayStoreClient *playstore.Client
	// GooglePlayReceiptValidator is the googleplay receipt validator for v1 apis
	GooglePlayReceiptValidator receiptvalidator.Validator
	// AppleAppStoreReceiptValidator is the apple store receipt validator for v1 apis
	AppleAppStoreReceiptValidator receiptvalidator.Validator
	// BraintreePaymentFunction is the lambda function that provides the Node.js braintree payment gateway
	BraintreePaymentFunction cloudfunctions.Function
	// PayPalPaymentFunction is the lambda function that provides access to the paypal payment gateway
	PayPalPaymentFunction cloudfunctions.Function

	// PaymentSlackMessageService is a service sending messages to #
	PaymentSlackMessageService *services.SlackMessageService

	// MessageSendQueue is the message send queue.
	MessageSendQueue sendmessagequeue.Queue
)

// Verify verifies if all variables have been properly set.
func Verify() {
	if AccessTokenValidator == nil {
		panic(errors.New("The access token validator has not been set"))
	}
	if AccountDatabase == nil {
		panic(errors.New("The account database has not been set"))
	}
	if ProductDatabase == nil {
		panic(errors.New("The product database has not been set"))
	}
	if ProductAccessService == nil {
		panic(errors.New("The product access service has not been set"))
	}
	if PassAccessService == nil {
		panic(errors.New("The pass access service has not been set"))
	}
	if ProductService == nil {
		panic(errors.New("The product service has not been set"))
	}
	if PassService == nil {
		panic(errors.New("The pass service has not been set"))
	}
	if TransactionService == nil {
		panic(errors.New("The transcation service has not been set"))
	}
	if TransactionServiceV2 == nil {
		panic(errors.New("The transcation service has not been set"))
	}
	if StoreProductService == nil {
		panic(errors.New("The store product service has not been set"))
	}
	if BraintreePaymentFunction == nil {
		panic(errors.New("The braintree payment function has not been set"))
	}
	if PayPalPaymentFunction == nil {
		panic(errors.New("The paypal payment function has not been set"))
	}
	if MessageSendQueue == nil {
		panic(errors.New("The message send queue has not been set"))
	}
}
