package handlers2

import (
	"context"
	"encoding/json"

	"bitbucket.org/calmisland/go-server-requests/apierrors"
	"bitbucket.org/calmisland/go-server-requests/apirequests"
	"bitbucket.org/calmisland/go-server-utils/textutils"
	"bitbucket.org/calmisland/payment-lambda-funcs/src/iap"
	"github.com/awa/go-iap/playstore"
)

type debugReceiptAndroidRequestBody struct {
	Receipt   string `json:"receipt"`
	Signature string `json:"signature"`
}

type debugReceiptAndroidResponseBody struct {
	IsValid     bool                     `json:"isValid"`
	ReceiptInfo iap.PlayStoreReceiptJSON `json:"receiptInfo"`
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
	var respBody debugReceiptAndroidResponseBody = debugReceiptAndroidResponseBody{
		IsValid:     isValid,
		ReceiptInfo: objReceipt,
	}

	resp.SetBody(&respBody)

	return nil
}
