package v2

import (
	"encoding/json"
	"net/http"

	"bitbucket.org/calmisland/go-server-requests/apierrors"
	"bitbucket.org/calmisland/go-server-requests/apirequests"
	"bitbucket.org/calmisland/go-server-utils/textutils"
	"bitbucket.org/calmisland/payment-lambda-funcs/internal/global"
	"bitbucket.org/calmisland/payment-lambda-funcs/internal/helpers"
	"bitbucket.org/calmisland/payment-lambda-funcs/internal/services/v1/iap"
	"github.com/awa/go-iap/playstore"
	"github.com/labstack/echo/v4"
	"google.golang.org/api/androidpublisher/v3"
)

type debugReceiptAndroidSubscriptionRequestBody struct {
	Receipt   string `json:"receipt"`
	Signature string `json:"signature"`
}

type debugReceiptAndroidSubscriptionResponseBody struct {
	IsValid          bool                                  `json:"isValid"`
	ReceiptInfo      iap.PlayStoreReceiptJSON              `json:"receiptInfo"`
	SubscriptionInfo androidpublisher.SubscriptionPurchase `json:"subscriptionInfo"`
}

// DebugReceiptAndroidSubscription handles receipt process requests.
func DebugReceiptAndroidSubscription(c echo.Context) error {
	// Parse the request body

	reqBody := new(debugReceiptAndroidSubscriptionRequestBody)
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
		return helpers.HandleInternalError(c, err)
	}

	isValid, err := playstore.VerifySignature(iap.GetService().GetAndroidPublicKey(objReceipt.PackageName), []byte(reqBody.Receipt), signature)

	if err != nil {
		return helpers.HandleInternalError(c, err)
	}

	subscriptionInfo, err := global.GooglePlayStoreClient.VerifySubscription(c.Request().Context(), objReceipt.PackageName, objReceipt.ProductID, objReceipt.PurchaseToken)

	if err != nil {
		return helpers.HandleInternalError(c, err)
	}

	var respBody debugReceiptAndroidSubscriptionResponseBody = debugReceiptAndroidSubscriptionResponseBody{
		IsValid:     isValid,
		ReceiptInfo: objReceipt,
	}

	if err == nil {
		respBody.SubscriptionInfo = *subscriptionInfo
	}

	return c.JSON(http.StatusOK, respBody)
}
