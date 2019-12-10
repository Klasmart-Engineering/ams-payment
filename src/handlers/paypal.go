package handlers

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"time"

	"bitbucket.org/calmisland/go-server-account/transactions"
	"bitbucket.org/calmisland/go-server-cloud/cloudfunctions"
	"bitbucket.org/calmisland/go-server-logs/logger"
	"bitbucket.org/calmisland/go-server-messages/messages"
	"bitbucket.org/calmisland/go-server-messages/messagetemplates"
	"bitbucket.org/calmisland/go-server-requests/apierrors"
	"bitbucket.org/calmisland/go-server-requests/apirequests"
	"bitbucket.org/calmisland/go-server-utils/timeutils"
	"bitbucket.org/calmisland/payment-lambda-funcs/src/globals"
	"bitbucket.org/calmisland/payment-lambda-funcs/src/services"
)

func purchasePermissions(accountID string, productCode string) (bool, error) {
	switch productCode {
	case "com.calmid.learnandplay.blp.standard":
		//If the user has access to a premium pass, block purchase of a standard pass.
		access, err := globals.PassAccessService.GetPassAccessVOByAccountIDPassID(accountID, "com.calmid.learnandplay.blp.premium")
		if err != nil {
			return false, err
		}
		if access != nil {
			return false, nil
		}
		return true, nil
	case "com.calmid.learnandplay.blp.premium", "com.calmid.badanamu.esl.premium":
		fallthrough
	default:
		return true, nil
	}
}

func createTransactionItem(ItemID string) *services.TransactionItem {
	now := timeutils.EpochMSNow()
	const oneYear = 365 * 24 * time.Hour
	expiration := now.Add(oneYear)
	return &services.TransactionItem{
		ItemID:         ItemID,
		StartDate:      now,
		ExpirationDate: expiration,
	}
}

type paypalPaymentRequestBody struct {
	OrderID     string `json:"orderId"`
	ProductCode string `json:"productCode"`
}

func HandlePayPalPayment(_ context.Context, req *apirequests.Request, resp *apirequests.Response) error {
	var reqBody paypalPaymentRequestBody
	err := req.UnmarshalBody(&reqBody)
	if err != nil {
		return resp.SetClientError(apierrors.ErrorBadRequestBody)
	}

	accountID := req.Session.Data.AccountID
	allowed, err := purchasePermissions(accountID, reqBody.ProductCode)
	if err != nil {
		return resp.SetServerError(err)
	}

	if !allowed {
		return resp.SetClientError(apierrors.ErrorBadRequestBody)
	}

	item := createTransactionItem(reqBody.ProductCode)
	passVO, err := globals.PassService.GetPassVOByPassID(item.ItemID)
	if err != nil {
		return resp.SetServerError(err)
	} else if passVO == nil {
		return resp.SetClientError(apierrors.ErrorBadRequestBody)
	}

	response, err := makePaypalPayment(reqBody.OrderID)
	if err != nil {
		return resp.SetServerError(err)
	}
	if !response.Success || fmt.Sprint(passVO.Price) != response.Value {
		return resp.SetClientError(apierrors.ErrorBadRequestBody)
	}
	transactionCode := services.TransactionCode{
		Store: transactions.PayPal,
		ID:    reqBody.OrderID,
	}
	items := []*services.TransactionItem{item}
	err = globals.TransactionService.SaveTransactionUnlockPasses(accountID, &transactionCode, items)
	if err != nil {
		return resp.SetServerError(err)
	}

	// Send an email once a pass is unlocked
	accountInfo, err := globals.AccountDatabase.GetAccountInfo(accountID)
	if err != nil {
		return resp.SetServerError(err)
	} else if accountInfo == nil {
		return resp.SetClientError(apierrors.ErrorItemNotFound)
	}
	userEmail := accountInfo.Email
	userLanguage := accountInfo.Language
	emailMessage := &messages.Message{
		MessageType: messages.MessageTypeEmail,
		Priority:    messages.MessagePriorityEmailNormal,
		Recipient:   userEmail,
		Language:    userLanguage,
		Template:    &messagetemplates.PassPurchasedTemplate{},
	}
	err = globals.MessageSendQueue.EnqueueMessage(emailMessage)
	if err != nil {
		return resp.SetServerError(err)
	}

	resp.SetBody(response)
	return nil
}

type paypalPaymentRequest struct {
	OrderID string `json:"orderId"`
}

type paypalLambdaPaymentResponse struct {
	Success bool   `json:"success"`
	Value   string `json:"value"`
}

func makePaypalPayment(orderID string) (*paypalLambdaPaymentResponse, error) {
	input := paypalPaymentRequest{
		OrderID: orderID,
	}
	payload, err := json.Marshal(input)
	if err != nil {
		return nil, err
	}
	result, err := invokePaypalLambda(payload)
	if err != nil {
		return nil, err
	}
	if result.IsError {
		logger.LogFormat("[PAYPAL] Lambda invocation returned an error: [%s]\n", result.Payload)
		return nil, errors.New("Payment error")
	}
	var lambdaResponse paypalLambdaPaymentResponse
	err = json.Unmarshal(result.Payload, &lambdaResponse)
	if err != nil {
		return nil, err
	}

	return &lambdaResponse, err
}

func invokePaypalLambda(payload []byte) (*cloudfunctions.FunctionInvokeOutput, error) {
	invokeInput := cloudfunctions.FunctionInvokeInput{
		IsEvent:  false,
		IsDryRun: false,
		Payload:  payload,
	}
	return globals.PayPalPaymentFunction.Invoke(&invokeInput)
}
