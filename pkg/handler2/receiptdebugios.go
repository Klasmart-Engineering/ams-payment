package handlers2

import (
	"context"

	"bitbucket.org/calmisland/go-server-requests/apierrors"
	"bitbucket.org/calmisland/go-server-requests/apirequests"
	"bitbucket.org/calmisland/go-server-utils/textutils"
	"bitbucket.org/calmisland/payment-lambda-funcs/pkg/iap"
	"github.com/awa/go-iap/appstore"
)

type debugReceiptIosRequestBody struct {
	IsSubscription bool   `json:"isSubscription"`
	BundleID       string `json:"bundleId"`
	Receipt        string `json:"receipt"`
}

// DebugReceiptIos handles receipt process requests.
func DebugReceiptIos(ctx context.Context, req *apirequests.Request, resp *apirequests.Response) error {
	// Parse the request body
	var reqBody debugReceiptIosRequestBody
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
