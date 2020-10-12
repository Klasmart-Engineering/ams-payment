package handlers

import (
	"context"
	"encoding/json"
	"fmt"

	"bitbucket.org/calmisland/go-server-product/productaccessservice"
	"bitbucket.org/calmisland/go-server-product/storeproducts"
	"bitbucket.org/calmisland/go-server-requests/apierrors"
	"bitbucket.org/calmisland/go-server-requests/apirequests"
	"bitbucket.org/calmisland/go-server-utils/textutils"
	"bitbucket.org/calmisland/go-server-utils/timeutils"
	"bitbucket.org/calmisland/payment-lambda-funcs/src/globals"
	"bitbucket.org/calmisland/payment-lambda-funcs/src/iap"
	"bitbucket.org/calmisland/payment-lambda-funcs/src/services"

	"github.com/awa/go-iap/appstore"
	"github.com/awa/go-iap/playstore"
	"github.com/calmisland/go-errors"
	log "github.com/sirupsen/logrus"
)

type v2ReceiptIosRequestBody struct {
	IsSubscription bool   `json:"isSubscription"`
	BundleID       string `json:"bundleId"`
	TransactionID  string `json:"transactionId"`
	Receipt        string `json:"receipt"`
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

// v2HandlerProcessReceiptIos handles receipt process requests.
func v2HandlerProcessReceiptIos(ctx context.Context, req *apirequests.Request, resp *apirequests.Response) error {
	// Parse the request body
	var reqBody v2ReceiptIosRequestBody
	err := req.UnmarshalBody(&reqBody)
	if err != nil {
		return resp.SetClientError(apierrors.ErrorBadRequestBody)
	}

	transactionID := textutils.SanitizeString(reqBody.TransactionID)
	bundleID := textutils.SanitizeString(reqBody.BundleID)
	receipt := textutils.SanitizeMultiLineString(reqBody.Receipt)

	// fmt.Println(reqBody.IsSubscription)

	// Validate Input parameters
	if len(transactionID) == 0 {
		return resp.SetClientError(apierrors.ErrorInvalidParameters.WithField("transactionId"))
	}

	if len(bundleID) == 0 {
		return resp.SetClientError(apierrors.ErrorInvalidParameters.WithField("bundleID"))
	}

	if len(receipt) == 0 {
		return resp.SetClientError(apierrors.ErrorInvalidParameters.WithField("receipt"))
	}

	accountID := req.Session.Data.AccountID

	var transactionCode services.TransactionCode
	transactionCode.Store = "apple"
	transactionCode.ID = transactionID

	contextLogger := log.WithFields(log.Fields{
		"method":        "v2HandlerProcessReceiptIos",
		"accountID":     accountID,
		"transactionID": transactionID,
	})

	contextLogger.Info(reqBody)

	iapClient := appstore.New()

	iapReq := appstore.IAPRequest{
		ReceiptData: receipt,
	}

	if reqBody.IsSubscription {
		password := iap.GetService().GetIosSharedKey(bundleID)
		iapReq.Password = password
		// fmt.Println(password)
	}

	iapResp := &appstore.IAPResponse{}
	err = iapClient.Verify(ctx, iapReq, iapResp)

	if err != nil {
		resp.SetServerError(err)
	} else if iapResp.Status != 0 {
		err := appstore.HandleError(iapResp.Status)

		switch err {
		case appstore.ErrInvalidJSON, appstore.ErrInvalidReceiptData:
			logFormat(contextLogger, "[IAPPROCESSRECEIPT] Received a receipt with invalid format for store [apple] and transaction [%s]", transactionID)
			return resp.SetClientError(apierrors.ErrorIAPInvalidReceiptFormat)
		case appstore.ErrReceiptUnauthenticated, appstore.ErrReceiptUnauthorized:
			logFormat(contextLogger, "[IAPPROCESSRECEIPT] Received an unauthorized receipt for store [apple] and transaction [%s]", transactionID)
			return resp.SetClientError(apierrors.ErrorIAPReceiptUnauthorized)
		case appstore.ErrInvalidSharedSecret:
			logFormat(contextLogger, "[IAPPROCESSRECEIPT] Received an unauthorized receipt for store [apple] and transaction [%s]", transactionID)
			// return resp.SetClientError(&APIError{StatusCode: http.StatusForbidden, ErrorCode: 310, ErrorName: "IAP_RECEIPT_IOS_SHARED_SECRET_NOT_MATCHED"})
			return resp.SetClientError(apierrors.ErrorIAPReceiptUnauthorized)
		case appstore.ErrServerUnavailable:
			logFormat(contextLogger, "[IAPPROCESSRECEIPT] The ios verification server is currently unavailable.")
			return resp.SetClientError(apierrors.ErrorIAPServiceUnavailable)
		}

		return resp.SetServerError(err)
	}

	// fmt.Println(iapResp)

	productPurchase := findIosInAppInfoWithTransactionID(&iapResp.Receipt.InApp, transactionID)

	if productPurchase == nil {
		logFormat(contextLogger, "[IAPPROCESSRECEIPT] Unable to find transaction [%s] in receipt for store [apple]", transactionID)
		return resp.SetClientError(apierrors.ErrorIAPReceiptTransactionNotFound)
	}

	// Validating transaction
	transaction, err := globals.TransactionService.GetTransactionByTransactionCode(&transactionCode)
	if err != nil {
		return resp.SetServerError(err)
	} else if transaction != nil {
		if transaction.AccountID == accountID {
			logFormat(contextLogger, "[IAPPROCESSRECEIPT] The transaction [%s] for store [apple] has already been processed by the same account [%s].", transactionID, accountID)
			return resp.SetClientError(apierrors.ErrorIAPTransactionAlreadyProcessedByYou)
		}

		logFormat(contextLogger, "[IAPPROCESSRECEIPT] The transaction [%s] for store [apple] requested for account [%s] has already been processed by another account [%s].", transactionID, accountID, transaction.AccountID)
		return resp.SetClientError(apierrors.ErrorIAPTransactionAlreadyProcessed)
	}

	storeProductID := productPurchase.ProductID
	storeProducts, err := globals.StoreProductService.GetStoreProductVOListByStoreProductID(storeProductID)
	if err != nil {
		return resp.SetServerError(err)
	} else if len(storeProducts) == 0 {
		logFormat(contextLogger, "[IAPPROCESSRECEIPT] The transaction [%s] for store [apple] and store product [%s] isn't available for sale.", transactionID, storeProductID)
		return resp.SetClientError(apierrors.ErrorIAPProductNotForSale)
	}

	productType := storeproducts.StoreProductTypeDefault
	passItems := []*services.PassItem{}
	productItems := []*productaccessservice.ProductAccessVOItem{}

	timeNow := timeutils.EpochMSNow()
	for _, product := range storeProducts {
		if productType == storeproducts.StoreProductTypeDefault {
			productType = product.Type
		} else if productType != product.Type {
			return resp.SetServerError(errors.Errorf("Expected product type [%d] but found [%d] for store product ID: %s", productType, product.Type, storeProductID))
		}

		if product.Type == storeproducts.StoreProductTypePass {
			passInfo, err := globals.PassService.GetPassVOByPassID(product.ItemID)
			if err != nil {
				return resp.SetServerError(err)
			} else if passInfo == nil {
				return resp.SetServerError(errors.Errorf("Unable to retrieve information about pass [%s] when processing IAP receipt", product.ItemID))
			}

			passItems = append(passItems, &services.PassItem{
				PassID:    passInfo.PassID,
				Price:     passInfo.Price,
				Currency:  passInfo.Currency,
				StartDate: timeNow,
				Duration:  passInfo.Duration,
			})
		} else if product.Type == storeproducts.StoreProductTypeProduct {
			productItems = append(productItems, &productaccessservice.ProductAccessVOItem{
				ProductID: product.ItemID,
				StartDate: timeNow,
			})
		}
	}

	if productType == storeproducts.StoreProductTypeProduct {
		err = globals.TransactionService.SaveTransactionUnlockProducts(accountID, &transactionCode, productItems)
		if err != nil {
			return resp.SetServerError(err)
		}
	} else if productType == storeproducts.StoreProductTypePass {
		err = globals.TransactionService.SaveTransactionUnlockPasses(accountID, &transactionCode, passItems)
		if err != nil {
			return resp.SetServerError(err)
		}
	}

	logFormat(contextLogger, "[IAPPROCESSRECEIPT] Successfully processed transaction [%s] for store [apple], store product [%s] and account [%s].", transactionID, storeProductID, accountID)
	return nil
}

type v2ReceiptAndroidRequestBody struct {
	Receipt   string `json:"receipt"`
	Signature string `json:"signature"`
}

// v2HandlerProcessReceiptAndroid handles receipt process requests.
func v2HandlerProcessReceiptAndroid(ctx context.Context, req *apirequests.Request, resp *apirequests.Response) error {
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

	accountID := req.Session.Data.AccountID

	transactionID := objReceipt.OrderID

	var transactionCode services.TransactionCode
	transactionCode.Store = "apple"
	transactionCode.ID = objReceipt.OrderID

	contextLogger := log.WithFields(log.Fields{
		"method":        "v2HandlerProcessReceiptIos",
		"accountID":     accountID,
		"transactionID": objReceipt.OrderID,
	})

	contextLogger.Info(reqBody)

	publicKey, hasAppPublicKey := iap.GetService().AndroidPublicKeys[objReceipt.PackageName]

	if !hasAppPublicKey {
		errMessage := fmt.Sprintf("Failed to verify Google Play Store receipt for unsupported application: %s", objReceipt.PackageName)
		logFormat(contextLogger, errMessage)
		return resp.SetClientError(apierrors.ErrorIAPReceiptUnauthorized)
	}

	isValid, err := playstore.VerifySignature(publicKey, []byte(reqBody.Receipt), signature)

	if err != nil {
		return resp.SetServerError(err)
	}

	if !isValid {
		logFormat(contextLogger, "The Google Play Store receipt is not valid (signature fail)")
		return resp.SetClientError(apierrors.ErrorIAPReceiptUnauthorized)
	}

	productPurchase := objReceipt

	// Validating transaction
	transaction, err := globals.TransactionService.GetTransactionByTransactionCode(&transactionCode)
	if err != nil {
		return resp.SetServerError(err)
	} else if transaction != nil {
		if transaction.AccountID == accountID {
			logFormat(contextLogger, "[IAPPROCESSRECEIPT] The transaction [%s] for store [android] has already been processed by the same account [%s].", transactionID, accountID)
			return resp.SetClientError(apierrors.ErrorIAPTransactionAlreadyProcessedByYou)
		}

		logFormat(contextLogger, "[IAPPROCESSRECEIPT] The transaction [%s] for store [android] requested for account [%s] has already been processed by another account [%s].", transactionID, accountID, transaction.AccountID)
		return resp.SetClientError(apierrors.ErrorIAPTransactionAlreadyProcessed)
	}

	storeProductID := productPurchase.ProductID
	storeProducts, err := globals.StoreProductService.GetStoreProductVOListByStoreProductID(storeProductID)
	if err != nil {
		return resp.SetServerError(err)
	} else if len(storeProducts) == 0 {
		logFormat(contextLogger, "[IAPPROCESSRECEIPT] The transaction [%s] for store [android] and store product [%s] isn't available for sale.", transactionID, storeProductID)
		return resp.SetClientError(apierrors.ErrorIAPProductNotForSale)
	}

	productType := storeproducts.StoreProductTypeDefault
	passItems := []*services.PassItem{}
	productItems := []*productaccessservice.ProductAccessVOItem{}

	timeNow := timeutils.EpochMSNow()
	for _, product := range storeProducts {
		if productType == storeproducts.StoreProductTypeDefault {
			productType = product.Type
		} else if productType != product.Type {
			return resp.SetServerError(errors.Errorf("Expected product type [%d] but found [%d] for store product ID: %s", productType, product.Type, storeProductID))
		}

		if product.Type == storeproducts.StoreProductTypePass {
			passInfo, err := globals.PassService.GetPassVOByPassID(product.ItemID)
			if err != nil {
				return resp.SetServerError(err)
			} else if passInfo == nil {
				return resp.SetServerError(errors.Errorf("Unable to retrieve information about pass [%s] when processing IAP receipt", product.ItemID))
			}

			passItems = append(passItems, &services.PassItem{
				PassID:    passInfo.PassID,
				Price:     passInfo.Price,
				Currency:  passInfo.Currency,
				StartDate: timeNow,
				Duration:  passInfo.Duration,
			})
		} else if product.Type == storeproducts.StoreProductTypeProduct {
			productItems = append(productItems, &productaccessservice.ProductAccessVOItem{
				ProductID: product.ItemID,
				StartDate: timeNow,
			})
		}
	}

	if productType == storeproducts.StoreProductTypeProduct {
		err = globals.TransactionService.SaveTransactionUnlockProducts(accountID, &transactionCode, productItems)
		if err != nil {
			return resp.SetServerError(err)
		}
	} else if productType == storeproducts.StoreProductTypePass {
		err = globals.TransactionService.SaveTransactionUnlockPasses(accountID, &transactionCode, passItems)
		if err != nil {
			return resp.SetServerError(err)
		}
	}

	logFormat(contextLogger, "[IAPPROCESSRECEIPT] Successfully processed transaction [%s] for store [android], store product [%s] and account [%s].", transactionID, storeProductID, accountID)
	return nil
}
