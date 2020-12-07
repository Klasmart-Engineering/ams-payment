package handlers2

import (
	"context"
	"encoding/json"
	"fmt"

	"bitbucket.org/calmisland/go-server-product/productaccessservice"
	"bitbucket.org/calmisland/go-server-product/storeproducts"
	"bitbucket.org/calmisland/go-server-requests/apierrors"
	"bitbucket.org/calmisland/go-server-requests/apirequests"
	"bitbucket.org/calmisland/go-server-utils/textutils"
	"bitbucket.org/calmisland/go-server-utils/timeutils"
	"bitbucket.org/calmisland/payment-lambda-funcs/src/globals"
	"bitbucket.org/calmisland/payment-lambda-funcs/src/iap"
	services_v2 "bitbucket.org/calmisland/payment-lambda-funcs/src/services/v2"
	"bitbucket.org/calmisland/payment-lambda-funcs/src/utils"

	"github.com/awa/go-iap/playstore"
	"github.com/calmisland/go-errors"
	log "github.com/sirupsen/logrus"
)

type processReceiptAndroidRequestBody struct {
	Receipt   string `json:"receipt"`
	Signature string `json:"signature"`
}

// ProcessReceiptAndroid handles receipt process requests.
func ProcessReceiptAndroid(ctx context.Context, req *apirequests.Request, resp *apirequests.Response) error {
	// Parse the request body
	var reqBody processReceiptAndroidRequestBody
	err := req.UnmarshalBody(&reqBody)
	if err != nil {
		return resp.SetClientError(apierrors.ErrorBadRequestBody)
	}

	receipt := textutils.SanitizeMultiLineString(reqBody.Receipt)
	signature := textutils.SanitizeString(reqBody.Signature)

	if len(receipt) == 0 {
		return resp.SetClientError(apierrors.ErrorInvalidParameters.WithField("receipt"))
	}

	if len(signature) == 0 {
		return resp.SetClientError(apierrors.ErrorInvalidParameters.WithField("signature"))
	}

	var objReceipt iap.PlayStoreReceiptJSON
	err = json.Unmarshal([]byte(reqBody.Receipt), &objReceipt)

	if err != nil {
		return resp.SetServerError(err)
	}

	accountID := req.Session.Data.AccountID

	transactionID := objReceipt.OrderID

	var transactionCode services_v2.TransactionCode
	transactionCode.Store = "googlePlay"
	transactionCode.ID = objReceipt.OrderID

	contextLogger := log.WithFields(log.Fields{
		"paymentMethod": "InApp: googlePlay - v2",
		"accountID":     accountID,
		"transactionID": transactionID,
	})
	contextLogger.Info(reqBody)

	publicKey, hasAppPublicKey := iap.GetService().AndroidPublicKeys[objReceipt.PackageName]

	if !hasAppPublicKey {
		errMessage := fmt.Sprintf("Failed to verify Google Play Store receipt for unsupported application: %s", objReceipt.PackageName)
		utils.LogFormat(contextLogger, errMessage)
		return resp.SetClientError(apierrors.ErrorIAPReceiptUnauthorized)
	}

	isValid, err := playstore.VerifySignature(publicKey, []byte(reqBody.Receipt), signature)

	if err != nil {
		return resp.SetServerError(err)
	}

	if !isValid {
		utils.LogFormat(contextLogger, "The Google Play Store receipt is not valid (signature fail)")
		return resp.SetClientError(apierrors.ErrorIAPReceiptUnauthorized)
	}

	productPurchase := objReceipt

	// Validating transaction
	transaction, err := globals.TransactionServiceV2.GetTransactionByTransactionCode(&transactionCode)
	if err != nil {
		return resp.SetServerError(err)
	} else if transaction != nil {
		if transaction.AccountID == accountID {
			utils.LogFormat(contextLogger, "[IAPPROCESSRECEIPT] The transaction [%s] for store [android] has already been processed by the same account [%s].", transactionID, accountID)
			return resp.SetClientError(apierrors.ErrorIAPTransactionAlreadyProcessedByYou)
		}

		utils.LogFormat(contextLogger, "[IAPPROCESSRECEIPT] The transaction [%s] for store [android] requested for account [%s] has already been processed by another account [%s].", transactionID, accountID, transaction.AccountID)
		return resp.SetClientError(apierrors.ErrorIAPTransactionAlreadyProcessed)
	}

	storeProductID := productPurchase.ProductID
	storeProducts, err := globals.StoreProductService.GetStoreProductVOListByStoreProductID(storeProductID)
	if err != nil {
		return resp.SetServerError(err)
	} else if len(storeProducts) == 0 {
		utils.LogFormat(contextLogger, "[IAPPROCESSRECEIPT] The transaction [%s] for store [android] and store product [%s] isn't available for sale.", transactionID, storeProductID)
		return resp.SetClientError(apierrors.ErrorIAPProductNotForSale)
	}

	productType := storeproducts.StoreProductTypeDefault
	passItems := []*services_v2.PassItem{}
	productItems := []*productaccessservice.ProductAccessVOItem{}

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
				return resp.SetServerError(errors.Errorf("Unable to retrieve information about pass [%s] when processing IAP receipt", product.ItemID))
			}

			passItems = append(passItems, &services_v2.PassItem{
				PassID:        passInfo.PassID,
				Price:         passInfo.Price,
				Currency:      passInfo.Currency,
				StartDate:     timeNow,
				ExpiresDateMS: timeNow + 10000, // TODO: fix this value
			})
		} else if product.Type == storeproducts.StoreProductTypeProduct {
			productItems = append(productItems, &productaccessservice.ProductAccessVOItem{
				ProductID: product.ItemID,
				StartDate: timeNow,
			})
		}
	}

	if productType == storeproducts.StoreProductTypeProduct {
		err = globals.TransactionServiceV2.SaveTransactionUnlockProducts(accountID, &transactionCode, productItems)
		if err != nil {
			return resp.SetServerError(err)
		}
	} else if productType == storeproducts.StoreProductTypePass {
		err = globals.TransactionServiceV2.SaveTransactionUnlockPasses(accountID, &transactionCode, passItems)
		if err != nil {
			return resp.SetServerError(err)
		}
	}

	utils.LogFormat(contextLogger, "[IAPPROCESSRECEIPT] Successfully processed transaction [%s] for store [android], store product [%s] and account [%s].", transactionID, storeProductID, accountID)
	return nil
}
