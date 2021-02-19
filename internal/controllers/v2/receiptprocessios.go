package v2

import (
	"strconv"

	"bitbucket.org/calmisland/go-server-requests/apierrors"
	"bitbucket.org/calmisland/go-server-requests/apirequests"
	"bitbucket.org/calmisland/go-server-utils/textutils"
	"bitbucket.org/calmisland/payment-lambda-funcs/internal/global"
	"bitbucket.org/calmisland/payment-lambda-funcs/internal/helpers"
	utils "bitbucket.org/calmisland/payment-lambda-funcs/internal/helpers"
	"bitbucket.org/calmisland/payment-lambda-funcs/internal/services/v1/iap"
	services_v2 "bitbucket.org/calmisland/payment-lambda-funcs/internal/services/v2/iap"
	"github.com/awa/go-iap/appstore"
	"github.com/labstack/echo/v4"
	log "github.com/sirupsen/logrus"
)

type processReceiptIosRequestBody struct {
	BundleID      string `json:"bundleId"`
	TransactionID string `json:"transactionId"`
	Receipt       string `json:"receipt"`
}

// ProcessReceiptIos handles receipt process requests.
func ProcessReceiptIos(c echo.Context) error {
	accountID := helpers.GetAccountID(c)

	// Parse the request body
	reqBody := new(processReceiptIosRequestBody)
	err := c.Bind(reqBody)
	if err != nil {
		return apirequests.EchoSetClientError(c, apierrors.ErrorBadRequestBody)
	}

	transactionID := textutils.SanitizeString(reqBody.TransactionID)
	bundleID := textutils.SanitizeString(reqBody.BundleID)
	receipt := textutils.SanitizeMultiLineString(reqBody.Receipt)

	// fmt.Println(reqBody.IsSubscription)

	// Validate Input parameters
	if len(transactionID) == 0 {
		return apirequests.EchoSetClientError(c, apierrors.ErrorInvalidParameters.WithField("transactionId"))
	}

	if len(bundleID) == 0 {
		return apirequests.EchoSetClientError(c, apierrors.ErrorInvalidParameters.WithField("bundleID"))
	}

	if len(receipt) == 0 {
		return apirequests.EchoSetClientError(c, apierrors.ErrorInvalidParameters.WithField("receipt"))
	}

	contextLogger := log.WithFields(log.Fields{
		"paymentMethod": "InApp: apple - v2",
		"accountID":     accountID,
		"transactionID": reqBody.TransactionID,
	})

	contextLogger.Info(reqBody)

	iapClient := appstore.New()

	password, hasSharedKey := iap.GetService().IosSharedSecrects[bundleID]
	if hasSharedKey == false {
		return apirequests.EchoSetClientError(c, apierrors.ErrorInvalidParameters.WithField("bundleID").WithMessage("No Shared Key"))
	}

	iapReq := appstore.IAPRequest{
		ReceiptData: receipt,
		Password:    password,
	}

	iapResp := &appstore.IAPResponse{}
	err = iapClient.Verify(c.Request().Context(), iapReq, iapResp)

	if err != nil {
		return utils.HandleInternalError(c, err)
	} else if iapResp.Status != 0 {
		err := appstore.HandleError(iapResp.Status)

		switch err {
		case appstore.ErrInvalidJSON, appstore.ErrInvalidReceiptData:
			utils.LogFormat(contextLogger, "[IAPPROCESSRECEIPT] Received a receipt with invalid format for store [apple] and transaction [%s]", transactionID)
			return apirequests.EchoSetClientError(c, apierrors.ErrorIAPInvalidReceiptFormat)
		case appstore.ErrReceiptUnauthenticated, appstore.ErrReceiptUnauthorized:
			utils.LogFormat(contextLogger, "[IAPPROCESSRECEIPT] Received an unauthorized receipt for store [apple] and transaction [%s]", transactionID)
			return apirequests.EchoSetClientError(c, apierrors.ErrorIAPReceiptUnauthorized)
		case appstore.ErrInvalidSharedSecret:
			utils.LogFormat(contextLogger, "[IAPPROCESSRECEIPT] Received an unauthorized receipt for store [apple] and transaction [%s]", transactionID)
			// return apirequests.EchoSetClientError(c, &APIError{StatusCode: http.StatusForbidden, ErrorCode: 310, ErrorName: "IAP_RECEIPT_IOS_SHARED_SECRET_NOT_MATCHED"})
			return apirequests.EchoSetClientError(c, apierrors.ErrorIAPReceiptUnauthorized)
		case appstore.ErrServerUnavailable:
			utils.LogFormat(contextLogger, "[IAPPROCESSRECEIPT] The ios verification server is currently unavailable.")
			return apirequests.EchoSetClientError(c, apierrors.ErrorIAPServiceUnavailable)
		}

		return utils.HandleInternalError(c, err)
	}

	// fmt.Println(iapResp)

	productPurchase := findIosInAppInfoWithTransactionID(&iapResp.Receipt.InApp, transactionID)
	if productPurchase == nil {
		utils.LogFormat(contextLogger, "[IAPPROCESSRECEIPT] Unable to find transaction [%s] in receipt for store [apple]", transactionID)
		return apirequests.EchoSetClientError(c, apierrors.ErrorIAPReceiptTransactionNotFound)
	}

	contextLogger = contextLogger.WithFields(log.Fields{
		"IsTrialPeriod": productPurchase.IsTrialPeriod,
		"ExpiresDateMS": productPurchase.ExpiresDateMS,
	})

	var transactionCode services_v2.TransactionCode
	transactionCode.Store = "apple"
	transactionCode.ID = transactionID

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

	expireDateMS, err := strconv.Atoi(productPurchase.ExpiresDateMS)

	if err != nil {
		return utils.HandleInternalError(c, err)
	}

	purchaseDateMS, err := strconv.Atoi(productPurchase.PurchaseDateMS)

	if err != nil {
		return utils.HandleInternalError(c, err)
	}

	err = global.TransactionServiceV2.RegisterTransaction(accountID, storeProductID, transactionCode, storeProducts, int64(purchaseDateMS), int64(expireDateMS))

	if err != nil {
		return utils.HandleInternalError(c, err)
	}

	utils.LogFormat(contextLogger, "[IAPPROCESSRECEIPT] Successfully processed transaction [%s] for store [apple], store product [%s] and account [%s].", transactionID, storeProductID, accountID)
	return nil
}

// findIosInAppInfoWithTransactionID attempts to find a product purchase with a specific transaction ID.
func findIosInAppInfoWithTransactionID(inApps *[]appstore.InApp, transactionID string) *appstore.InApp {
	for _, inApp := range *inApps {
		if inApp.TransactionID == transactionID {
			return &inApp
		}
	}

	return nil
}
