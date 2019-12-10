package handlers

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"

	"bitbucket.org/calmisland/go-server-account/transactions"
	"bitbucket.org/calmisland/go-server-cloud/cloudfunctions"
	"bitbucket.org/calmisland/go-server-logs/logger"
	"bitbucket.org/calmisland/go-server-messages/messages"
	"bitbucket.org/calmisland/go-server-messages/messagetemplates"
	"bitbucket.org/calmisland/go-server-requests/apierrors"
	"bitbucket.org/calmisland/go-server-requests/apirequests"
	"bitbucket.org/calmisland/payment-lambda-funcs/src/globals"
	"bitbucket.org/calmisland/payment-lambda-funcs/src/services"
)

func HandleBraintreeToken(_ context.Context, req *apirequests.Request, resp *apirequests.Response) error {
	token, err := getBraintreeToken()
	if err != nil {
		resp.SetServerError(err)
		return err
	}

	resp.SetBody(token)
	return nil
}

type braintreeTokenResponseBody struct {
	Token string `json:"clientToken"`
}

func getBraintreeToken() (*braintreeTokenResponseBody, error) {
	payload := []byte("{\"type\":\"token\"}")
	result, err := invokeBraintreeLambda(payload)
	if err != nil {
		return nil, err
	}

	var lambdaResponse braintreeTokenResponseBody
	err = json.Unmarshal(result.Payload, &lambdaResponse)
	if err != nil {
		return nil, err
	}
	return &lambdaResponse, err
}

type braintreePaymentRequestBody struct {
	Nonce       string `json:"nonce"`
	ProductCode string `json:"productCode"`
}

func HandleBraintreePayment(_ context.Context, req *apirequests.Request, resp *apirequests.Response) error {
	var reqBody braintreePaymentRequestBody
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

	response, err := makeBraintreePayment(reqBody.Nonce, fmt.Sprint(passVO.Price))
	if err != nil {
		return resp.SetServerError(err)
	}
	transactionCode := services.TransactionCode{
		Store: transactions.BrainTree,
		ID:    response.TransactionId,
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

type braintreePaymentRequest struct {
	Type   string `json:"type"`
	Nonce  string `json:"nonce"`
	Amount string `json:"amount"`
}

type braintreeLambdaPaymentResponse struct {
	TransactionId string `json:"transactionId"`
}

func makeBraintreePayment(nonce string, amount string) (*braintreeLambdaPaymentResponse, error) {
	input := braintreePaymentRequest{
		Type:   "payment",
		Nonce:  nonce,
		Amount: amount,
	}
	payload, err := json.Marshal(input)
	if err != nil {
		return nil, err
	}
	result, err := invokeBraintreeLambda(payload)
	if err != nil {
		return nil, err
	}
	if result.IsError {
		logger.LogFormat("[BRAINTREE] Lambda invocation returned an error: [%s]\n", result.Payload)
		return nil, errors.New("Payment error")
	}
	var lambdaResponse braintreeLambdaPaymentResponse
	err = json.Unmarshal(result.Payload, &lambdaResponse)
	if err != nil {
		return nil, err
	}

	return &lambdaResponse, err
}

func invokeBraintreeLambda(payload []byte) (*cloudfunctions.FunctionInvokeOutput, error) {
	invokeInput := cloudfunctions.FunctionInvokeInput{
		IsEvent:  false,
		IsDryRun: false,
		Payload:  payload,
	}
	return globals.BraintreePaymentFunction.Invoke(&invokeInput)
}
