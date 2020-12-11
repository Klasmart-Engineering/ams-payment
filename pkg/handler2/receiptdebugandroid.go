package handlers2

import (
	"context"
	"encoding/json"

	"bitbucket.org/calmisland/go-server-requests/apierrors"
	"bitbucket.org/calmisland/go-server-requests/apirequests"
	"bitbucket.org/calmisland/go-server-utils/textutils"
	"bitbucket.org/calmisland/payment-lambda-funcs/pkg/iap"
	subscription "bitbucket.org/calmisland/payment-lambda-funcs/pkg/iap/android"
	"github.com/awa/go-iap/playstore"
	"google.golang.org/api/androidpublisher/v3"
)

type debugReceiptAndroidRequestBody struct {
	Receipt   string `json:"receipt"`
	Signature string `json:"signature"`
}

type debugReceiptAndroidResponseBody struct {
	IsValid          bool                                  `json:"isValid"`
	ReceiptInfo      iap.PlayStoreReceiptJSON              `json:"receiptInfo"`
	SubscriptionInfo androidpublisher.SubscriptionPurchase `json:"subscriptionInfo"`
}

// DebugReceiptAndroid handles receipt process requests.
func DebugReceiptAndroid(ctx context.Context, req *apirequests.Request, resp *apirequests.Response) error {
	// Parse the request body
	var reqBody debugReceiptAndroidRequestBody
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

	isValid, err := playstore.VerifySignature(iap.GetService().GetAndroidPublicKey(objReceipt.PackageName), []byte(reqBody.Receipt), signature)

	if err != nil {
		return resp.SetServerError(err)
	}

	subscriptionInfo, err := subscription.GetSubscriptionInformation(objReceipt.PackageName, objReceipt.ProductID, objReceipt.PurchaseToken)

	if err != nil {
		return resp.SetServerError(err)
	}

	var respBody debugReceiptAndroidResponseBody = debugReceiptAndroidResponseBody{
		IsValid:          isValid,
		ReceiptInfo:      objReceipt,
		SubscriptionInfo: *subscriptionInfo,
	}

	resp.SetBody(&respBody)

	return nil
}
