package handlers

import (
	"context"
	"strings"
	"time"

	"bitbucket.org/calmisland/go-server-account/transactions"
	"bitbucket.org/calmisland/go-server-iap/receiptvalidator"
	"bitbucket.org/calmisland/go-server-product/storeproducts"
	"bitbucket.org/calmisland/go-server-requests/apierrors"
	"bitbucket.org/calmisland/go-server-requests/apirequests"
	"bitbucket.org/calmisland/go-server-utils/timeutils"
	"bitbucket.org/calmisland/payment-lambda-funcs/src/globals"
	"bitbucket.org/calmisland/payment-lambda-funcs/src/services"
	"github.com/calmisland/go-errors"
)

const (
	appleStoreID      = "apple"
	googlePlayStoreID = "googlePlay"

	oneDay = time.Hour * 24
)

type processReceiptRequestBody struct {
	StoreID       string `json:"storeId"`
	TransactionID string `json:"transactionId"`
	Receipt       string `json:"receipt"`
}

// IsReceiptToAppleStore -validate receipt from Apple Store
func IsReceiptToAppleStore(platform string) bool {
	return strings.EqualFold(platform, appleStoreID)
}

// IsReceiptToGooglePlay -validate receipt from Google Store
func IsReceiptToGooglePlay(platform string) bool {
	return strings.EqualFold(platform, googlePlayStoreID)
}

// HandleProcessReceipt handles receipt process requests.
func HandleProcessReceipt(ctx context.Context, req *apirequests.Request, resp *apirequests.Response) error {
	// Parse the request body
	var reqBody processReceiptRequestBody
	err := req.UnmarshalBody(&reqBody)
	if err != nil {
		return resp.SetClientError(apierrors.ErrorBadRequestBody)
	}

	transactionID := reqBody.TransactionID
	storeID := reqBody.StoreID
	receipt := reqBody.Receipt

	// Validate Input parameters
	if len(transactionID) == 0 {
		resp.SetClientError(apierrors.ErrorInvalidParameters.WithField("transactionId"))
	} else if len(storeID) == 0 {
		resp.SetClientError(apierrors.ErrorInvalidParameters.WithField("storeId"))
	} else if len(receipt) == 0 {
		resp.SetClientError(apierrors.ErrorInvalidParameters.WithField("receipt"))
	}

	accountID := req.Session.Data.AccountID

	var transactionCode services.TransactionCode
	transactionCode.ID = transactionID

	var receiptValidator receiptvalidator.Validator
	if IsReceiptToGooglePlay(storeID) {
		transactionCode.Store = transactions.GooglePlay
		receiptValidator = globals.GooglePlayReceiptValidator
	} else if IsReceiptToAppleStore(storeID) {
		transactionCode.Store = transactions.Apple
		receiptValidator = globals.AppleAppStoreReceiptValidator
	} else {
		return resp.SetClientError(apierrors.ErrorInvalidParameters.WithField("storeId"))
	}

	// If no receipt validator is available
	if receiptValidator == nil {
		return resp.SetClientError(apierrors.ErrorIAPServiceUnavailable)
	}

	validatedReceipt, err := receiptValidator.ValidateReceipt(ctx, receipt)
	if err != nil {
		if validationErr, isValidationError := err.(receiptvalidator.ValidationError); isValidationError && validationErr != nil {
			switch validationErr.Code() {
			case receiptvalidator.ErrorCodeInvalidFormat:
				return resp.SetClientError(apierrors.ErrorIAPInvalidReceiptFormat)
			case receiptvalidator.ErrorCodeInvalidReceipt:
				return resp.SetClientError(apierrors.ErrorIAPReceiptUnauthorized)
			case receiptvalidator.ErrorCodeNetworkError, receiptvalidator.ErrorCodeServerUnavailable:
				return resp.SetClientError(apierrors.ErrorIAPServiceUnavailable)
			}
		}
		return resp.SetServerError(err)
	} else if validatedReceipt == nil {
		return resp.SetServerError(errors.Errorf("Received nil receipt after receipt validation for store: %s", storeID))
	}

	productPurchase := validatedReceipt.FindProductPurchaseWithTransactionID(transactionID)
	if productPurchase == nil {
		return resp.SetClientError(apierrors.ErrorIAPReceiptTransactionNotFound)
	}

	// Validating transaction
	transaction, err := globals.TransactionService.GetTransactionByTransactionCode(&transactionCode)
	if err != nil {
		return resp.SetServerError(err)
	} else if transaction != nil {
		if transaction.AccountID == accountID {
			return resp.SetClientError(apierrors.ErrorIAPTransactionAlreadyProcessedByYou)
		}
		return resp.SetClientError(apierrors.ErrorIAPTransactionAlreadyProcessed)
	}

	storeProductID := productPurchase.ProductID
	storeProducts, err := globals.StoreProductService.GetStoreProductVOListByStoreProductID(storeProductID)
	if err != nil {
		return resp.SetServerError(err)
	} else if len(storeProducts) == 0 {
		return resp.SetClientError(apierrors.ErrorIAPProductNotForSale)
	}

	productType := storeproducts.StoreProductTypeDefault
	transactionItems := make([]*services.TransactionItem, 0, len(storeProducts))

	timeNow := timeutils.EpochMSNow()
	for _, product := range storeProducts {
		if productType == storeproducts.StoreProductTypeDefault {
			productType = product.Type
		} else if productType != product.Type {
			return resp.SetServerError(errors.Errorf("Expected product type [%d] but found [%d] for store product ID: %s", productType, product.Type, storeProductID))
		}

		if product.Type == storeproducts.StoreProductTypePass {
			passInfo, err := globals.PassService.GetPassVOByPassID(product.ItemID)
			if err != nil {
				return resp.SetServerError(err)
			} else if passInfo == nil {
				return resp.SetClientError(apierrors.ErrorItemNotFound)
			}

			expirationDate := timeNow.Add(time.Duration(passInfo.Duration) * oneDay)
			transactionItems = append(transactionItems, &services.TransactionItem{
				ItemID:         passInfo.PassID,
				StartDate:      timeNow,
				ExpirationDate: expirationDate,
			})
		} else if product.Type == storeproducts.StoreProductTypeProduct {
			transactionItems = append(transactionItems, &services.TransactionItem{
				ItemID:    product.ItemID,
				StartDate: timeNow,
			})
		}
	}

	if productType == storeproducts.StoreProductTypeProduct {
		err = globals.TransactionService.SaveTransactionUnlockProducts(accountID, &transactionCode, transactionItems)
		if err != nil {
			return resp.SetServerError(err)
		}
	} else if productType == storeproducts.StoreProductTypePass {
		err = globals.TransactionService.SaveTransactionUnlockPasses(accountID, &transactionCode, transactionItems)
		if err != nil {
			return resp.SetServerError(err)
		}
	}

	return nil
}
