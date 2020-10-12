package handlers

import (
	"context"
	"encoding/json"

	"bitbucket.org/calmisland/go-server-requests/apierrors"
	"bitbucket.org/calmisland/go-server-requests/apirequests"
	"bitbucket.org/calmisland/go-server-utils/textutils"
	"bitbucket.org/calmisland/payment-lambda-funcs/src/iap"

	"github.com/awa/go-iap/appstore"
	"github.com/awa/go-iap/playstore"
)

type v2ReceiptDebugIosRequestBody struct {
	IsSubscription bool   `json:"isSubscription"`
	BundleID       string `json:"bundleId"`
	Receipt        string `json:"receipt"`
}

// v2HandlerProcessReceiptIos handles receipt process requests.
func v2HandlerDebugReceiptIos(ctx context.Context, req *apirequests.Request, resp *apirequests.Response) error {
	// Parse the request body
	var reqBody v2ReceiptDebugIosRequestBody
	err := req.UnmarshalBody(&reqBody)
	if err != nil {
		return resp.SetClientError(apierrors.ErrorBadRequestBody)
	}

	bundleID := textutils.SanitizeString(reqBody.BundleID)
	receipt := textutils.SanitizeMultiLineString(reqBody.Receipt)

	// fmt.Println(reqBody.IsSubscription)

	if len(bundleID) == 0 {
		return resp.SetClientError(apierrors.ErrorInvalidParameters.WithField("bundleID"))
	}

	if len(receipt) == 0 {
		return resp.SetClientError(apierrors.ErrorInvalidParameters.WithField("receipt"))
	}

	iapClient := appstore.New()

	iapReq := appstore.IAPRequest{
		ReceiptData: receipt,
	}

	if reqBody.IsSubscription {
		password := iap.GetService().GetIosSharedKey(bundleID)
		iapReq.Password = password
	}

	iapResp := &appstore.IAPResponse{}
	err = iapClient.Verify(ctx, iapReq, iapResp)

	if err != nil {
		return resp.SetServerError(err)
	}

	resp.SetBody(&iapResp)

	return nil
}

type v2ReceiptDebugAndroidRequestBody struct {
	Receipt   string `json:"receipt"`
	Signature string `json:"signature"`
}

type v2ReceiptDebugAndroidResponseBody struct {
	IsValid     bool                     `json:"isValid"`
	ReceiptInfo iap.PlayStoreReceiptJSON `json:"receiptInfo"`
}

// v2HandlerProcessReceiptIos handles receipt process requests.
func v2HandlerDebugReceiptAndroid(ctx context.Context, req *apirequests.Request, resp *apirequests.Response) error {
	// Parse the request body
	var reqBody v2ReceiptDebugAndroidRequestBody
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
	var respBody v2ReceiptDebugAndroidResponseBody = v2ReceiptDebugAndroidResponseBody{
		IsValid:     isValid,
		ReceiptInfo: objReceipt,
	}

	resp.SetBody(&respBody)

	return nil
}
