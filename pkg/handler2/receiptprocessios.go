package handlers2

import (
	"context"
	"strconv"

	"bitbucket.org/calmisland/go-server-product/productaccessservice"
	"bitbucket.org/calmisland/go-server-product/storeproducts"
	"bitbucket.org/calmisland/go-server-requests/apierrors"
	"bitbucket.org/calmisland/go-server-requests/apirequests"
	"bitbucket.org/calmisland/go-server-utils/textutils"
	"bitbucket.org/calmisland/go-server-utils/timeutils"
	"bitbucket.org/calmisland/payment-lambda-funcs/pkg/global"
	"bitbucket.org/calmisland/payment-lambda-funcs/pkg/iap"
	services_v2 "bitbucket.org/calmisland/payment-lambda-funcs/pkg/service2"
	utils "bitbucket.org/calmisland/payment-lambda-funcs/pkg/util"
	"github.com/awa/go-iap/appstore"
	"github.com/calmisland/go-errors"
	log "github.com/sirupsen/logrus"
)

type processReceiptIosRequestBody struct {
	BundleID      string `json:"bundleId"`
	TransactionID string `json:"transactionId"`
	Receipt       string `json:"receipt"`
}

// ProcessReceiptIos handles receipt process requests.
func ProcessReceiptIos(ctx context.Context, req *apirequests.Request, resp *apirequests.Response) error {
	// Parse the request body
	var reqBody processReceiptIosRequestBody
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

	var transactionCode services_v2.TransactionCode
	transactionCode.Store = "apple"
	transactionCode.ID = transactionID

	contextLogger := log.WithFields(log.Fields{
		"paymentMethod": "InApp: apple - v2",
		"accountID":     accountID,
		"transactionID": transactionID,
	})

	contextLogger.Info(reqBody)

	iapClient := appstore.New()

	password, hasSharedKey := iap.GetService().IosSharedSecrects[bundleID]
	if hasSharedKey == false {
		return resp.SetClientError(apierrors.ErrorInvalidParameters.WithField("bundleID").WithMessage("No Shared Key"))
	}

	iapReq := appstore.IAPRequest{
		ReceiptData: receipt,
		Password:    password,
	}

	iapResp := &appstore.IAPResponse{}
	err = iapClient.Verify(ctx, iapReq, iapResp)

	if err != nil {
		resp.SetServerError(err)
	} else if iapResp.Status != 0 {
		err := appstore.HandleError(iapResp.Status)

		switch err {
		case appstore.ErrInvalidJSON, appstore.ErrInvalidReceiptData:
			utils.LogFormat(contextLogger, "[IAPPROCESSRECEIPT] Received a receipt with invalid format for store [apple] and transaction [%s]", transactionID)
			return resp.SetClientError(apierrors.ErrorIAPInvalidReceiptFormat)
		case appstore.ErrReceiptUnauthenticated, appstore.ErrReceiptUnauthorized:
			utils.LogFormat(contextLogger, "[IAPPROCESSRECEIPT] Received an unauthorized receipt for store [apple] and transaction [%s]", transactionID)
			return resp.SetClientError(apierrors.ErrorIAPReceiptUnauthorized)
		case appstore.ErrInvalidSharedSecret:
			utils.LogFormat(contextLogger, "[IAPPROCESSRECEIPT] Received an unauthorized receipt for store [apple] and transaction [%s]", transactionID)
			// return resp.SetClientError(&APIError{StatusCode: http.StatusForbidden, ErrorCode: 310, ErrorName: "IAP_RECEIPT_IOS_SHARED_SECRET_NOT_MATCHED"})
			return resp.SetClientError(apierrors.ErrorIAPReceiptUnauthorized)
		case appstore.ErrServerUnavailable:
			utils.LogFormat(contextLogger, "[IAPPROCESSRECEIPT] The ios verification server is currently unavailable.")
			return resp.SetClientError(apierrors.ErrorIAPServiceUnavailable)
		}

		return resp.SetServerError(err)
	}

	// fmt.Println(iapResp)

	productPurchase := findIosInAppInfoWithTransactionID(&iapResp.Receipt.InApp, transactionID)

	contextLogger = contextLogger.WithFields(log.Fields{
		"IsTrialPeriod": productPurchase.IsTrialPeriod,
		"ExpiresDateMS": productPurchase.ExpiresDateMS,
	})

	if productPurchase == nil {
		utils.LogFormat(contextLogger, "[IAPPROCESSRECEIPT] Unable to find transaction [%s] in receipt for store [apple]", transactionID)
		return resp.SetClientError(apierrors.ErrorIAPReceiptTransactionNotFound)
	}

	// Validating transaction
	transaction, err := global.TransactionServiceV2.GetTransactionByTransactionCode(&transactionCode)
	if err != nil {
		return resp.SetServerError(err)
	} else if transaction != nil {
		if transaction.AccountID == accountID {
			utils.LogFormat(contextLogger, "[IAPPROCESSRECEIPT] The transaction [%s] for store [apple] has already been processed by the same account [%s].", transactionID, accountID)
			return resp.SetClientError(apierrors.ErrorIAPTransactionAlreadyProcessedByYou)
		}

		utils.LogFormat(contextLogger, "[IAPPROCESSRECEIPT] The transaction [%s] for store [apple] requested for account [%s] has already been processed by another account [%s].", transactionID, accountID, transaction.AccountID)
		return resp.SetClientError(apierrors.ErrorIAPTransactionAlreadyProcessed)
	}

	storeProductID := productPurchase.ProductID
	storeProducts, err := global.StoreProductService.GetStoreProductVOListByStoreProductID(storeProductID)
	if err != nil {
		return resp.SetServerError(err)
	} else if len(storeProducts) == 0 {
		utils.LogFormat(contextLogger, "[IAPPROCESSRECEIPT] The transaction [%s] for store [apple] and store product [%s] isn't available for sale.", transactionID, storeProductID)
		return resp.SetClientError(apierrors.ErrorIAPProductNotForSale)
	}

	productType := storeproducts.StoreProductTypeDefault
	passItems := []*services_v2.PassItem{}
	productItems := []*productaccessservice.ProductAccessVOItem{}

	timeNow := timeutils.EpochMSNow()
	for _, product := range storeProducts {
		contextLogger.Info(product)
		if productType == storeproducts.StoreProductTypeDefault {
			productType = product.Type
		} else if productType != product.Type {
			return resp.SetServerError(errors.Errorf("Expected product type [%d] but found [%d] for store product ID: %s", productType, product.Type, storeProductID))
		}

		if product.Type == storeproducts.StoreProductTypePass {
			passInfo, err := global.PassService.GetPassVOByPassID(product.ItemID)
			contextLogger.Info(passInfo)
			if err != nil {
				return resp.SetServerError(err)
			} else if passInfo == nil {
				return resp.SetServerError(errors.Errorf("Unable to retrieve information about pass [%s] when processing IAP receipt", product.ItemID))
			}

			expireDateMS, err := strconv.Atoi(productPurchase.ExpiresDateMS)

			if err != nil {
				return resp.SetServerError(err)
			}

			purchaseDateMS, err := strconv.Atoi(productPurchase.PurchaseDateMS)

			if err != nil {
				return resp.SetServerError(err)
			}

			passItems = append(passItems, &services_v2.PassItem{
				PassID:        passInfo.PassID,
				Price:         passInfo.Price,
				Currency:      passInfo.Currency,
				StartDate:     timeutils.EpochTimeMS(purchaseDateMS),
				ExpiresDateMS: timeutils.EpochTimeMS(expireDateMS),
			})
			contextLogger.Info(passItems)
		} else if product.Type == storeproducts.StoreProductTypeProduct {
			productItems = append(productItems, &productaccessservice.ProductAccessVOItem{
				ProductID: product.ItemID,
				StartDate: timeNow,
			})
		}
	}

	if productType == storeproducts.StoreProductTypeProduct {
		err = global.TransactionServiceV2.SaveTransactionUnlockProducts(accountID, &transactionCode, productItems)
		if err != nil {
			return resp.SetServerError(err)
		}
	} else if productType == storeproducts.StoreProductTypePass {
		err = global.TransactionServiceV2.SaveTransactionUnlockPasses(accountID, &transactionCode, passItems)
		if err != nil {
			return resp.SetServerError(err)
		}
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
