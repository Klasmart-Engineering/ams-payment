package v2

import (
	"fmt"
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

	bundleID := textutils.SanitizeString(reqBody.BundleID)
	receipt := textutils.SanitizeMultiLineString(reqBody.Receipt)

	// fmt.Println(reqBody.IsSubscription)

	if len(bundleID) == 0 {
		return apirequests.EchoSetClientError(c, apierrors.ErrorInvalidParameters.WithField("bundleID"))
	}

	if len(receipt) == 0 {
		return apirequests.EchoSetClientError(c, apierrors.ErrorInvalidParameters.WithField("receipt"))
	}

	contextLogger := log.WithFields(log.Fields{
		"paymentMethod": "InApp: apple - v2",
		"accountID":     accountID,
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
			utils.LogFormat(contextLogger, "[IAPPROCESSRECEIPT] Received a receipt with invalid format for store [apple]")
			return apirequests.EchoSetClientError(c, apierrors.ErrorIAPInvalidReceiptFormat)
		case appstore.ErrReceiptUnauthenticated, appstore.ErrReceiptUnauthorized:
			utils.LogFormat(contextLogger, "[IAPPROCESSRECEIPT] Received an unauthorized receipt for store [apple]")
			return apirequests.EchoSetClientError(c, apierrors.ErrorIAPReceiptUnauthorized)
		case appstore.ErrInvalidSharedSecret:
			utils.LogFormat(contextLogger, "[IAPPROCESSRECEIPT] Received an unauthorized receipt for store [apple]")
			// return apirequests.EchoSetClientError(c, &APIError{StatusCode: http.StatusForbidden, ErrorCode: 310, ErrorName: "IAP_RECEIPT_IOS_SHARED_SECRET_NOT_MATCHED"})
			return apirequests.EchoSetClientError(c, apierrors.ErrorIAPReceiptUnauthorized)
		case appstore.ErrServerUnavailable:
			utils.LogFormat(contextLogger, "[IAPPROCESSRECEIPT] The ios verification server is currently unavailable.")
			return apirequests.EchoSetClientError(c, apierrors.ErrorIAPServiceUnavailable)
		}

		return utils.HandleInternalError(c, err)
	}

	// fmt.Println(iapResp)
	latestProduct := getLatestIosInAppInfo(&iapResp.Receipt.InApp)
	if latestProduct == nil {
		fmt.Println("[IAPPROCESSRECEIPT] No latest App Info")
		return apirequests.EchoSetClientError(c, apierrors.ErrorIAPReceiptTransactionNotFound)
	}

	transactionID := latestProduct.TransactionID

	transactionExists, transactionExistsErr := checkIfPurchaseExists(accountID, latestProduct)
	if transactionExistsErr != nil {
		fmt.Println("[IAPPROCESSRECEIPT] transactionExistsErr")
		return apirequests.EchoSetClientError(c, apierrors.ErrorIAPReceiptTransactionNotFound)
	}

	if transactionExists {
		return nil
	}

	contextLogger = contextLogger.WithFields(log.Fields{
		"IsTrialPeriod": latestProduct.IsTrialPeriod,
		"ExpiresDateMS": latestProduct.ExpiresDateMS,
	})

	var transactionCode services_v2.TransactionCode
	transactionCode.Store = "apple"
	transactionCode.ID = transactionID

	storeProductID := latestProduct.ProductID

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

	expireDateMS, err := strconv.Atoi(latestProduct.ExpiresDateMS)

	if err != nil {
		return utils.HandleInternalError(c, err)
	}

	purchaseDateMS, err := strconv.Atoi(latestProduct.PurchaseDateMS)

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

/*
	InApp struct {
		Quantity                    string `json:"quantity"`
		ProductID                   string `json:"product_id"`
		TransactionID               string `json:"transaction_id"`
		OriginalTransactionID       string `json:"original_transaction_id"`
		WebOrderLineItemID          string `json:"web_order_line_item_id,omitempty"`
		PromotionalOfferID          string `json:"promotional_offer_id"`
		SubscriptionGroupIdentifier string `json:"subscription_group_identifier"`
		OfferCodeRefName            string `json:"offer_code_ref_name,omitempty"`

		IsTrialPeriod        string `json:"is_trial_period"`
		IsInIntroOfferPeriod string `json:"is_in_intro_offer_period,omitempty"`
		IsUpgraded           string `json:"is_upgraded,omitempty"`

		ExpiresDate

		PurchaseDate
		OriginalPurchaseDate

		CancellationDate
		CancellationReason string `json:"cancellation_reason,omitempty"`
	}

		// The PurchaseDate type indicates the date and time that the item was purchased
	PurchaseDate struct {
		PurchaseDate    string `json:"purchase_date"`
		PurchaseDateMS  string `json:"purchase_date_ms"`
		PurchaseDatePST string `json:"purchase_date_pst"`
	}

	// The OriginalPurchaseDate type indicates the beginning of the subscription period
	OriginalPurchaseDate struct {
		OriginalPurchaseDate    string `json:"original_purchase_date"`
		OriginalPurchaseDateMS  string `json:"original_purchase_date_ms"`
		OriginalPurchaseDatePST string `json:"original_purchase_date_pst"`
	}

	// The ExpiresDate type indicates the expiration date for the subscription
	ExpiresDate struct {
		ExpiresDate             string `json:"expires_date,omitempty"`
		ExpiresDateMS           string `json:"expires_date_ms,omitempty"`
		ExpiresDatePST          string `json:"expires_date_pst,omitempty"`
		ExpiresDateFormatted    string `json:"expires_date_formatted,omitempty"`
		ExpiresDateFormattedPST string `json:"expires_date_formatted_pst,omitempty"`
	}

*/

func getLatestIosInAppInfo(inApps *[]appstore.InApp) *appstore.InApp {
	if len(*inApps) == 0 {
		return nil
	}
	var latest *appstore.InApp = nil
	var latestIdx = 0

	fmt.Println("[IAPPROCESSRECEIPT] getLatestIosInAppInfo ")
	for idx, inApp := range *inApps {

		fmt.Println(inApp)
		if latest == nil {
			latestIdx = 0
			latest = &inApp
			continue
		}

		newExpires, err := strconv.Atoi((*inApps)[idx].ExpiresDate.ExpiresDateMS)
		if err != nil {
			return nil
		}
		latestExpires, err := strconv.Atoi((*inApps)[latestIdx].ExpiresDate.ExpiresDateMS)
		if err != nil {
			return nil
		}

		fmt.Println("latestExpires: ", latestExpires)
		fmt.Println("newExpires: ", newExpires)
		if latestExpires < newExpires {
			latest = &inApp
			latestIdx = idx
			continue
		}
	}

	return &(*inApps)[latestIdx]
}

func checkIfPurchaseExists(accountId string, inApps *appstore.InApp) (bool, error) {

	var transactionCode services_v2.TransactionCode
	transactionCode.Store = "apple"
	transactionCode.ID = inApps.TransactionID

	transaction, err := global.TransactionServiceV2.GetTransaction(accountId, &transactionCode)
	if err != nil {
		return false, err
	}
	if transaction != nil {
		return true, nil
	}
	return false, nil
}
