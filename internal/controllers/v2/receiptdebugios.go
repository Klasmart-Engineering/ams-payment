package v2

import (
	"net/http"

	"bitbucket.org/calmisland/go-server-requests/apierrors"
	"bitbucket.org/calmisland/go-server-requests/apirequests"
	"bitbucket.org/calmisland/go-server-utils/textutils"
	"bitbucket.org/calmisland/payment-lambda-funcs/internal/services/v1/iap"
	"github.com/awa/go-iap/appstore"
	"github.com/labstack/echo/v4"
)

type debugReceiptIosRequestBody struct {
	IsSubscription bool   `json:"isSubscription"`
	BundleID       string `json:"bundleId"`
	Receipt        string `json:"receipt"`
}

// DebugReceiptIos handles receipt process requests.
func DebugReceiptIos(c echo.Context) error {
	// Parse the request body
	reqBody := new(debugReceiptIosRequestBody)
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

	iapClient := appstore.New()

	iapReq := appstore.IAPRequest{
		ReceiptData: receipt,
	}

	if reqBody.IsSubscription {
		password := iap.GetService().GetIosSharedKey(bundleID)
		iapReq.Password = password
	}

	iapResp := &appstore.IAPResponse{}
	err = iapClient.Verify(c.Request().Context(), iapReq, iapResp)

	if err != nil {
		return err
	}

	return c.JSON(http.StatusOK, iapResp)
}
