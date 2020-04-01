package handlers

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log"

	"bitbucket.org/calmisland/go-server-account/transactions"
	"bitbucket.org/calmisland/go-server-cloud/cloudfunctions"
	"bitbucket.org/calmisland/go-server-logs/logger"
	"bitbucket.org/calmisland/go-server-messages/messages"
	"bitbucket.org/calmisland/go-server-messages/messagetemplates"
	"bitbucket.org/calmisland/go-server-product/passes"
	"bitbucket.org/calmisland/go-server-requests/apierrors"
	"bitbucket.org/calmisland/go-server-requests/apirequests"
	"bitbucket.org/calmisland/go-server-utils/timeutils"
	"bitbucket.org/calmisland/payment-lambda-funcs/src/globals"
	"bitbucket.org/calmisland/payment-lambda-funcs/src/services"
)

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

	passVO, err := globals.PassService.GetPassVOByPassID(reqBody.ProductCode)
	if err != nil {
		return resp.SetServerError(err)
	} else if passVO == nil {
		return resp.SetClientError(apierrors.ErrorBadRequestBody)
	}

	item := &services.PassItem{
		PassID:    passVO.PassID,
		Price:     passVO.Price,
		Currency:  passVO.Currency,
		StartDate: timeutils.EpochMSNow(),
		Duration:  passes.DurationDays(passVO.Duration),
	}

	passPrice, err := passVO.Price.ToString(passVO.Currency)
	if err != nil {
		return resp.SetServerError(err)
	}
	response, err := makePaypalPayment(reqBody.OrderID)
	if err != nil {
		return resp.SetServerError(err)
	}
	if !response.Success || passPrice != response.Value {
		return resp.SetClientError(apierrors.ErrorBadRequestBody)
	}
	transactionCode := services.TransactionCode{
		Store: transactions.PayPal,
		ID:    reqBody.OrderID,
	}
	err = globals.TransactionService.SaveTransactionUnlockPasses(accountID, &transactionCode, []*services.PassItem{item})
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
	timeNow := timeutils.EpochMSNow()
	endPassValidityDate := timeutils.ConvEpochTimeMS(timeNow.Time().AddDate(0, 0, int(passVO.Duration)))
	userEmail := accountInfo.Email
	userLanguage := accountInfo.Language
	emailMessage := &messages.Message{
		MessageType: messages.MessageTypeEmail,
		Priority:    messages.MessagePriorityEmailNormal,
		Recipient:   userEmail,
		Language:    userLanguage,
		Template: &messagetemplates.PassPurchasedTemplate{
			LearningPass:   passVO.Title,
			ExpirationDate: fmt.Sprintf("%d/%d/%d", endPassValidityDate.Time().Year(), endPassValidityDate.Time().Month(), endPassValidityDate.Time().Day()),
		},
	}
	err = globals.MessageSendQueue.EnqueueMessage(emailMessage)
	if err != nil {
		return resp.SetServerError(err)
	}

	log.Printf("[PAYPALPAYMENT] A payment for the pass [%s] by account [%s] succeeded\n", passVO.PassID, accountID)

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
