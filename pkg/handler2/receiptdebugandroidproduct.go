package handlers2

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"os"

	"bitbucket.org/calmisland/go-server-requests/apierrors"
	"bitbucket.org/calmisland/go-server-requests/apirequests"
	"bitbucket.org/calmisland/go-server-utils/textutils"
	"bitbucket.org/calmisland/payment-lambda-funcs/pkg/iap"
	"github.com/awa/go-iap/playstore"
	"google.golang.org/api/androidpublisher/v3"
)

type debugReceiptAndroidProductRequestBody struct {
	Receipt   string `json:"receipt"`
	Signature string `json:"signature"`
}

type debugReceiptAndroidProductResponseBody struct {
	IsValid     bool                             `json:"isValid"`
	ReceiptInfo iap.PlayStoreReceiptJSON         `json:"receiptInfo"`
	ProductInfo androidpublisher.ProductPurchase `json:"ProductInfo"`
}

// DebugReceiptAndroidProduct handles receipt process requests.
func DebugReceiptAndroidProduct(ctx context.Context, req *apirequests.Request, resp *apirequests.Response) error {
	// Parse the request body
	var reqBody debugReceiptAndroidProductRequestBody
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

	jsonKeyBase64 := os.Getenv("GOOGLE_PLAYSTORE_JSON_KEY")
	jsonKeyStr, err := base64.StdEncoding.DecodeString(jsonKeyBase64)
	if err != nil {
		return resp.SetServerError(err)
	}
	jsonKey := []byte(jsonKeyStr)

	client, err := playstore.New(jsonKey)
	if err != nil {
		return resp.SetServerError(err)
	}

	ProductInfo, err := client.VerifyProduct(ctx, objReceipt.PackageName, objReceipt.ProductID, objReceipt.PurchaseToken)

	if err != nil {
		return resp.SetServerError(err)
	}

	var respBody debugReceiptAndroidProductResponseBody = debugReceiptAndroidProductResponseBody{
		IsValid:     isValid,
		ReceiptInfo: objReceipt,
	}

	if err == nil {
		respBody.ProductInfo = *ProductInfo
	}

	resp.SetBody(&respBody)

	return nil
}
