package v2

import (
	"encoding/base64"
	"encoding/json"
	"fmt"
	"os"

	"bitbucket.org/calmisland/go-server-requests/apierrors"
	"bitbucket.org/calmisland/go-server-requests/apirequests"
	"bitbucket.org/calmisland/go-server-utils/textutils"
	"bitbucket.org/calmisland/payment-lambda-funcs/internal/global"
	"bitbucket.org/calmisland/payment-lambda-funcs/internal/helpers"
	utils "bitbucket.org/calmisland/payment-lambda-funcs/internal/helpers"
	"bitbucket.org/calmisland/payment-lambda-funcs/internal/services/v1/iap"
	services_v2 "bitbucket.org/calmisland/payment-lambda-funcs/internal/services/v2/iap"
	"github.com/awa/go-iap/playstore"
	"github.com/labstack/echo/v4"
	log "github.com/sirupsen/logrus"
)

type processReceiptAndroidRequestBody struct {
	Receipt   string `json:"receipt"`
	Signature string `json:"signature"`
}

// ProcessReceiptAndroid handles receipt process requests.
func ProcessReceiptAndroid(c echo.Context) error {
	accountID := helpers.GetAccountID(c)

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
		return utils.HandleInternalError(c, err)
	}

	transactionID := objReceipt.OrderID

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
		return utils.HandleInternalError(c, err)
	}

	if !isValid {
		utils.LogFormat(contextLogger, "The Google Play Store receipt is not valid (signature fail)")
		return apirequests.EchoSetClientError(c, apierrors.ErrorIAPReceiptUnauthorized)
	}

	productPurchase := objReceipt

	jsonKeyBase64 := os.Getenv("GOOGLE_PLAYSTORE_JSON_KEY")
	jsonKeyStr, err := base64.StdEncoding.DecodeString(jsonKeyBase64)
	if err != nil {
		return utils.HandleInternalError(c, err)
	}
	jsonKey := []byte(jsonKeyStr)

	client, err := playstore.New(jsonKey)
	if err != nil {
		return utils.HandleInternalError(c, err)
	}

	subscriptionInfo, err := client.VerifySubscription(c.Request().Context(), objReceipt.PackageName, objReceipt.ProductID, objReceipt.PurchaseToken)

	if err != nil {
		return utils.HandleInternalError(c, err)
	}

	contextLogger = contextLogger.WithFields(log.Fields{
		"OrderId": subscriptionInfo.OrderId,
	})

	contextLogger.Info(subscriptionInfo)

	var transactionCode services_v2.TransactionCode
	transactionCode.Store = "googlePlay"
	transactionCode.ID = subscriptionInfo.OrderId

	storeProductID := productPurchase.ProductID

	storeProducts, err := global.TransactionServiceV2.ValidateTransaction(accountID, storeProductID, transactionCode)

	if te, ok := err.(*services_v2.ValidationTranscationAlreadyProcessedByAnotherAccountError); ok {
		utils.LogFormat(contextLogger, "[IAPPROCESSRECEIPT] The transaction [%s] for store [%s] requested for account [%s] has already been processed by another account [%s].", transactionID, transactionCode.Store, accountID, te.AnotherAccountID)
		return apirequests.EchoSetClientError(c, apierrors.ErrorIAPTransactionAlreadyProcessed)
	}

	switch err {
	case services_v2.ErrValidationNotFoundStoreProduct:
		utils.LogFormat(contextLogger, "[IAPPROCESSRECEIPT] The transaction [%s] for store [%s] and store product [%s] isn't available for sale.", transactionID, transactionCode.Store, storeProductID)
		return apirequests.EchoSetClientError(c, apierrors.ErrorIAPProductNotForSale)
	case services_v2.ErrValidationTranscationAlreadyExist:
		for _, product := range storeProducts {
			global.TransactionServiceV2.PassAccessService.DeletePassAccess(accountID, product.ItemID)
		}
		break
	case nil:
		break
	default:
		return utils.HandleInternalError(c, err)
	}

	err = global.TransactionServiceV2.RegisterTransaction(accountID, storeProductID, transactionCode, storeProducts, subscriptionInfo.StartTimeMillis, subscriptionInfo.ExpiryTimeMillis)

	if err != nil {
		return utils.HandleInternalError(c, err)
	}

	utils.LogFormat(contextLogger, "[IAPPROCESSRECEIPT] Successfully processed transaction [%s] for store [android], store product [%s] and account [%s].", transactionID, storeProductID, accountID)
	return nil
}
