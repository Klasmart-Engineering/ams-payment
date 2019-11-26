package globals

import (
	"bitbucket.org/calmisland/go-server-account/accountdatabase"
	"bitbucket.org/calmisland/go-server-product/passaccessservice"
	"bitbucket.org/calmisland/go-server-product/productaccessservice"
	"bitbucket.org/calmisland/go-server-product/productdatabase"
	"bitbucket.org/calmisland/go-server-requests/tokens/accesstokens"
	"bitbucket.org/calmisland/payment-lambda-funcs/src/services"
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
	// ProductAccessService allows use of the product database
	ProductAccessService *productaccessservice.StandardProductAccessService
	// PassAccessService allows use of the product database
	PassAccessService *passaccessservice.StandardPassAccessService
)

// Verify verifies if all variables have been properly set.
func Verify() {
	if AccessTokenValidator == nil {
		panic(errors.New("The access token validator has not been set"))
	}
	if ProductAccessService == nil {
		panic(errors.New("The product access service has not been set"))
	}
	if PassAccessService == nil {
		panic(errors.New("The pass access service has not been set"))
	}
}
