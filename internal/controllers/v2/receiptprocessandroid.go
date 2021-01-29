package v2

import (
	"encoding/base64"
	"encoding/json"
	"fmt"
	"os"

	"bitbucket.org/calmisland/go-server-auth/authmiddlewares"
	"bitbucket.org/calmisland/go-server-product/productaccessservice"
	"bitbucket.org/calmisland/go-server-product/storeproducts"
	"bitbucket.org/calmisland/go-server-requests/apierrors"
	"bitbucket.org/calmisland/go-server-requests/apirequests"
	"bitbucket.org/calmisland/go-server-utils/textutils"
	"bitbucket.org/calmisland/go-server-utils/timeutils"
	"bitbucket.org/calmisland/payment-lambda-funcs/internal/global"
	utils "bitbucket.org/calmisland/payment-lambda-funcs/internal/helpers"
	"bitbucket.org/calmisland/payment-lambda-funcs/internal/services/v1/iap"
	services_v2 "bitbucket.org/calmisland/payment-lambda-funcs/internal/services/v2"
	"github.com/awa/go-iap/playstore"
	"github.com/calmisland/go-errors"
	"github.com/labstack/echo/v4"
	log "github.com/sirupsen/logrus"
)

type processReceiptAndroidRequestBody struct {
	Receipt   string `json:"receipt"`
	Signature string `json:"signature"`
}

// ProcessReceiptAndroid handles receipt process requests.
func ProcessReceiptAndroid(c echo.Context) error {
	cc := c.(*authmiddlewares.AuthContext)
	accountID := cc.Session.Data.AccountID
	// Parse the request body
	reqBody := new(processReceiptAndroidRequestBody)
	err := c.Bind(reqBody)
	if err != nil {
		return apirequests.EchoSetClientError(c, apierrors.ErrorBadRequestBody)
	}

	receipt := textutils.SanitizeMultiLineString(reqBody.Receipt)
	signature := textutils.SanitizeString(reqBody.Signature)

	if len(receipt) == 0 {
		return apirequests.EchoSetClientError(c, apierrors.ErrorInvalidParameters.WithField("receipt"))
	}

	if len(signature) == 0 {
		return apirequests.EchoSetClientError(c, apierrors.ErrorInvalidParameters.WithField("signature"))
	}

	var objReceipt iap.PlayStoreReceiptJSON
	err = json.Unmarshal([]byte(reqBody.Receipt), &objReceipt)

	if err != nil {
		return err
	}

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
		return apirequests.EchoSetClientError(c, apierrors.ErrorIAPReceiptUnauthorized)
	}

	isValid, err := playstore.VerifySignature(publicKey, []byte(reqBody.Receipt), signature)

	if err != nil {
		return err
	}

	if !isValid {
		utils.LogFormat(contextLogger, "The Google Play Store receipt is not valid (signature fail)")
		return apirequests.EchoSetClientError(c, apierrors.ErrorIAPReceiptUnauthorized)
	}

	productPurchase := objReceipt

	// Validating transaction
	transaction, err := global.TransactionServiceV2.GetTransactionByTransactionCode(&transactionCode)
	if err != nil {
		return err
	} else if transaction != nil {
		if transaction.AccountID == accountID {
			utils.LogFormat(contextLogger, "[IAPPROCESSRECEIPT] The transaction [%s] for store [android] has already been processed by the same account [%s].", transactionID, accountID)
			return apirequests.EchoSetClientError(c, apierrors.ErrorIAPTransactionAlreadyProcessedByYou)
		}

		utils.LogFormat(contextLogger, "[IAPPROCESSRECEIPT] The transaction [%s] for store [android] requested for account [%s] has already been processed by another account [%s].", transactionID, accountID, transaction.AccountID)
		return apirequests.EchoSetClientError(c, apierrors.ErrorIAPTransactionAlreadyProcessed)
	}

	timeNow := timeutils.EpochMSNow()

	jsonKeyBase64 := os.Getenv("GOOGLE_PLAYSTORE_JSON_KEY")
	jsonKeyStr, err := base64.StdEncoding.DecodeString(jsonKeyBase64)
	if err != nil {
		return err
	}
	jsonKey := []byte(jsonKeyStr)

	client, err := playstore.New(jsonKey)
	if err != nil {
		return err
	}

	subscriptionInfo, err := client.VerifySubscription(c.Request().Context(), objReceipt.PackageName, objReceipt.ProductID, objReceipt.PurchaseToken)

	foundSubscriptionInfo := err == nil

	contextLogger = contextLogger.WithFields(log.Fields{
		"foundSubscriptionInfo": foundSubscriptionInfo,
	})

	if foundSubscriptionInfo {
		timeNow = timeutils.EpochTimeMS(subscriptionInfo.StartTimeMillis)
		contextLogger = contextLogger.WithFields(log.Fields{
			"OrderId": subscriptionInfo.OrderId,
		})

		transactionCode.ID = subscriptionInfo.OrderId

		contextLogger.Info(subscriptionInfo)
	}

	storeProductID := productPurchase.ProductID
	storeProducts, err := global.StoreProductService.GetStoreProductVOListByStoreProductID(storeProductID)
	if err != nil {
		return err
	} else if len(storeProducts) == 0 {
		utils.LogFormat(contextLogger, "[IAPPROCESSRECEIPT] The transaction [%s] for store [android] and store product [%s] isn't available for sale.", transactionID, storeProductID)
		return apirequests.EchoSetClientError(c, apierrors.ErrorIAPProductNotForSale)
	}

	productType := storeproducts.StoreProductTypeDefault
	passItems := []*services_v2.PassItem{}
	productItems := []*productaccessservice.ProductAccessVOItem{}

	for _, product := range storeProducts {
		if productType == storeproducts.StoreProductTypeDefault {
			productType = product.Type
		} else if productType != product.Type {
			return errors.Errorf("Expected product type [%d] but found [%d] for store product ID: %s", productType, product.Type, storeProductID)
		}

		if product.Type == storeproducts.StoreProductTypePass {
			passInfo, err := global.PassService.GetPassVOByPassID(product.ItemID)

			if err != nil {
				return err
			} else if passInfo == nil {
				return errors.Errorf("Unable to retrieve information about pass [%s] when processing IAP receipt", product.ItemID)
			}

			expiredTime := timeNow + timeutils.EpochTimeMS(passInfo.DurationMS)

			if foundSubscriptionInfo {
				expiredTime = timeutils.EpochTimeMS(subscriptionInfo.ExpiryTimeMillis)
			}

			passItems = append(passItems, &services_v2.PassItem{
				PassID:        passInfo.PassID,
				Price:         passInfo.Price,
				Currency:      passInfo.Currency,
				StartDate:     timeNow,
				ExpiresDateMS: expiredTime,
			})
			contextLogger.Info(passItems)
		} else if product.Type == storeproducts.StoreProductTypeProduct {
			productItems = append(productItems, &productaccessservice.ProductAccessVOItem{
				ProductID: product.ItemID,
				StartDate: timeutils.EpochTimeMS(timeNow),
			})
		}
	}

	if productType == storeproducts.StoreProductTypeProduct {
		err = global.TransactionServiceV2.SaveTransactionUnlockProducts(accountID, &transactionCode, productItems)
		if err != nil {
			return err
		}
	} else if productType == storeproducts.StoreProductTypePass {
		err = global.TransactionServiceV2.SaveTransactionUnlockPasses(accountID, &transactionCode, passItems)
		if err != nil {
			return err
		}
	}

	utils.LogFormat(contextLogger, "[IAPPROCESSRECEIPT] Successfully processed transaction [%s] for store [android], store product [%s] and account [%s].", transactionID, storeProductID, accountID)
	return nil
}
