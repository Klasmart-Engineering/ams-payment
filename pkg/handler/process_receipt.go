package handlers

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"strings"
	"time"

	log "github.com/sirupsen/logrus"

	"bitbucket.org/calmisland/go-server-account/transactions"
	"bitbucket.org/calmisland/go-server-iap/receiptvalidator"
	"bitbucket.org/calmisland/go-server-product/productaccessservice"
	"bitbucket.org/calmisland/go-server-product/storeproducts"
	"bitbucket.org/calmisland/go-server-requests/apierrors"
	"bitbucket.org/calmisland/go-server-requests/apirequests"
	"bitbucket.org/calmisland/go-server-utils/textutils"
	"bitbucket.org/calmisland/go-server-utils/timeutils"
	"bitbucket.org/calmisland/payment-lambda-funcs/pkg/global"
	"bitbucket.org/calmisland/payment-lambda-funcs/pkg/service"
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

	transactionID := textutils.SanitizeString(reqBody.TransactionID)
	storeID := textutils.SanitizeString(reqBody.StoreID)
	receipt := textutils.SanitizeMultiLineString(reqBody.Receipt)

	// Validate Input parameters
	if len(transactionID) == 0 {
		return resp.SetClientError(apierrors.ErrorInvalidParameters.WithField("transactionId"))
	} else if len(storeID) == 0 {
		return resp.SetClientError(apierrors.ErrorInvalidParameters.WithField("storeId"))
	} else if len(receipt) == 0 {
		return resp.SetClientError(apierrors.ErrorInvalidParameters.WithField("receipt"))
	}

	accountID := req.Session.Data.AccountID

	var transactionCode services.TransactionCode
	transactionCode.ID = transactionID

	contextLogger := log.WithFields(log.Fields{
		"paymentMethod": "InApp: " + storeID + " - v1",
		"accountID":     accountID,
		"transactionID": transactionID,
	})

	contextLogger.Info(reqBody)

	var receiptValidator receiptvalidator.Validator
	if IsReceiptToGooglePlay(storeID) {
		transactionCode.Store = transactions.GooglePlay
		receiptValidator = global.GooglePlayReceiptValidator
	} else if IsReceiptToAppleStore(storeID) {
		transactionCode.Store = transactions.Apple
		receiptValidator = global.AppleAppStoreReceiptValidator
	} else {
		return resp.SetClientError(apierrors.ErrorInvalidParameters.WithField("storeId"))
	}

	// If no receipt validator is available
	if receiptValidator == nil {
		logFormat(contextLogger, "[IAPPROCESSRECEIPT] There is no receipt validator for [%s], so the service is currently unavailable.", storeID)
		return resp.SetClientError(apierrors.ErrorIAPServiceUnavailable)
	}

	validatedReceipt, err := receiptValidator.ValidateReceipt(ctx, receipt)
	if err != nil {
		if validationErr, isValidationError := err.(receiptvalidator.ValidationError); isValidationError && validationErr != nil {
			switch validationErr.Code() {
			case receiptvalidator.ErrorCodeInvalidFormat:
				logFormat(contextLogger, "[IAPPROCESSRECEIPT] Received a receipt with invalid format for store [%s] and transaction [%s]", storeID, transactionID)
				return resp.SetClientError(apierrors.ErrorIAPInvalidReceiptFormat)
			case receiptvalidator.ErrorCodeInvalidReceipt:
				logFormat(contextLogger, "[IAPPROCESSRECEIPT] Received an unauthorized receipt for store [%s] and transaction [%s]", storeID, transactionID)
				return resp.SetClientError(apierrors.ErrorIAPReceiptUnauthorized)
			case receiptvalidator.ErrorCodeNetworkError, receiptvalidator.ErrorCodeServerUnavailable:
				logFormat(contextLogger, "[IAPPROCESSRECEIPT] The store [%s] is currently unavailable.", storeID)
				return resp.SetClientError(apierrors.ErrorIAPServiceUnavailable)
			}
		}
		return resp.SetServerError(err)
	} else if validatedReceipt == nil {
		return resp.SetServerError(errors.Errorf("Received nil receipt after receipt validation for store: %s", storeID))
	}

	productPurchase := validatedReceipt.FindProductPurchaseWithTransactionID(transactionID)
	if productPurchase == nil {
		logFormat(contextLogger, "[IAPPROCESSRECEIPT] Unable to find transaction [%s] in receipt for store [%s]", transactionID, storeID)
		return resp.SetClientError(apierrors.ErrorIAPReceiptTransactionNotFound)
	}

	// Validating transaction
	transaction, err := global.TransactionService.GetTransactionByTransactionCode(&transactionCode)
	if err != nil {
		return resp.SetServerError(err)
	} else if transaction != nil {
		if transaction.AccountID == accountID {
			logFormat(contextLogger, "[IAPPROCESSRECEIPT] The transaction [%s] for store [%s] has already been processed by the same account [%s].", transactionID, storeID, accountID)
			return resp.SetClientError(apierrors.ErrorIAPTransactionAlreadyProcessedByYou)
		}

		logFormat(contextLogger, "[IAPPROCESSRECEIPT] The transaction [%s] for store [%s] requested for account [%s] has already been processed by another account [%s].", transactionID, storeID, accountID, transaction.AccountID)
		return resp.SetClientError(apierrors.ErrorIAPTransactionAlreadyProcessed)
	}

	storeProductID := productPurchase.ProductID
	storeProducts, err := global.StoreProductService.GetStoreProductVOListByStoreProductID(storeProductID)
	if err != nil {
		return resp.SetServerError(err)
	} else if len(storeProducts) == 0 {
		logFormat(contextLogger, "[IAPPROCESSRECEIPT] The transaction [%s] for store [%s] and store product [%s] isn't available for sale.", transactionID, storeID, storeProductID)
		return resp.SetClientError(apierrors.ErrorIAPProductNotForSale)
	}

	productType := storeproducts.StoreProductTypeDefault
	passItems := []*services.PassItem{}
	productItems := []*productaccessservice.ProductAccessVOItem{}

	timeNow := timeutils.EpochMSNow()
	for _, product := range storeProducts {
		if productType == storeproducts.StoreProductTypeDefault {
			productType = product.Type
		} else if productType != product.Type {
			return resp.SetServerError(errors.Errorf("Expected product type [%d] but found [%d] for store product ID: %s", productType, product.Type, storeProductID))
		}

		if product.Type == storeproducts.StoreProductTypePass {
			passInfo, err := global.PassService.GetPassVOByPassID(product.ItemID)
			if err != nil {
				return resp.SetServerError(err)
			} else if passInfo == nil {
				return resp.SetServerError(errors.Errorf("Unable to retrieve information about pass [%s] when processing IAP receipt", product.ItemID))
			}

			passItems = append(passItems, &services.PassItem{
				PassID:    passInfo.PassID,
				Price:     passInfo.Price,
				Currency:  passInfo.Currency,
				StartDate: timeNow,
				Duration:  passInfo.Duration,
			})
		} else if product.Type == storeproducts.StoreProductTypeProduct {
			productItems = append(productItems, &productaccessservice.ProductAccessVOItem{
				ProductID: product.ItemID,
				StartDate: timeNow,
			})
		}
	}

	if productType == storeproducts.StoreProductTypeProduct {
		err = global.TransactionService.SaveTransactionUnlockProducts(accountID, &transactionCode, productItems)
		if err != nil {
			return resp.SetServerError(err)
		}
	} else if productType == storeproducts.StoreProductTypePass {
		err = global.TransactionService.SaveTransactionUnlockPasses(accountID, &transactionCode, passItems)
		if err != nil {
			return resp.SetServerError(err)
		}
	}

	logFormat(contextLogger, "[IAPPROCESSRECEIPT] Successfully processed transaction [%s] for store [%s], store product [%s] and account [%s].", transactionID, storeID, storeProductID, accountID)
	return nil
}

func logFormat(contextLogger *log.Entry, format string, args ...interface{}) {
	contextLogger.Infof(format, args...)

	jsonMap := contextLogger.Data
	jsonMap["env"] = os.Getenv("SERVER_STAGE")
	jsonMap["message"] = fmt.Sprintf(format, args...)

	jsonObj, err := json.Marshal(jsonMap)

	if err != nil {
		contextLogger.Errorf("JSON marshalling process failure for a slack message")
	}

	global.PaymentSlackMessageService.SendMessage(string(jsonObj))
}