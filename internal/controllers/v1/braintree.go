package v1

import (
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"net/http"

	"bitbucket.org/calmisland/go-server-account/transactions"
	"bitbucket.org/calmisland/go-server-auth/authmiddlewares"
	"bitbucket.org/calmisland/go-server-cloud/cloudfunctions"
	"bitbucket.org/calmisland/go-server-logs/logger"
	"bitbucket.org/calmisland/go-server-messages/messages"
	"bitbucket.org/calmisland/go-server-messages/messagetemplates"
	"bitbucket.org/calmisland/go-server-product/passes"
	"bitbucket.org/calmisland/go-server-requests/apierrors"
	"bitbucket.org/calmisland/go-server-requests/apirequests"
	"bitbucket.org/calmisland/go-server-utils/timeutils"
	"bitbucket.org/calmisland/payment-lambda-funcs/internal/global"
	"bitbucket.org/calmisland/payment-lambda-funcs/internal/helpers"
	services "bitbucket.org/calmisland/payment-lambda-funcs/internal/services/v1"
	"github.com/labstack/echo/v4"
)

func HandleBraintreeToken(c echo.Context) error {
	token, err := getBraintreeToken()
	if err != nil {
		return helpers.HandleInternalError(c, err)
	}

	return c.JSON(http.StatusOK, token)
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

func HandleBraintreePayment(c echo.Context) error {
	cc := c.(*authmiddlewares.AuthContext)
	accountID := cc.Session.Data.AccountID

	reqBody := new(braintreePaymentRequestBody)
	err := c.Bind(reqBody)

	if err != nil {
		return apirequests.EchoSetClientError(c, apierrors.ErrorBadRequestBody)
	}

	passVO, err := global.PassService.GetPassVOByPassID(reqBody.ProductCode)
	if err != nil {
		return helpers.HandleInternalError(c, err)
	} else if passVO == nil {
		return apirequests.EchoSetClientError(c, apierrors.ErrorBadRequestBody)
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
		return helpers.HandleInternalError(c, err)
	}
	response, err := makeBraintreePayment(reqBody.Nonce, passPrice)
	if err != nil {
		return helpers.HandleInternalError(c, err)
	}
	transactionCode := services.TransactionCode{
		Store: transactions.BrainTree,
		ID:    response.TransactionId,
	}
	err = global.TransactionService.SaveTransactionUnlockPasses(accountID, &transactionCode, []*services.PassItem{item})
	if err != nil {
		return helpers.HandleInternalError(c, err)
	}

	// Send an email once a pass is unlocked
	accountInfo, err := global.AccountDatabase.GetAccountInfo(accountID)
	if err != nil {
		return helpers.HandleInternalError(c, err)
	} else if accountInfo == nil {
		return apirequests.EchoSetClientError(c, apierrors.ErrorItemNotFound)
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
	err = global.MessageSendQueue.EnqueueMessage(emailMessage)
	if err != nil {
		return helpers.HandleInternalError(c, err)
	}

	log.Printf("[BRAINTREEPAYMENT] A payment for the pass [%s] by account [%s] succeeded\n", passVO.PassID, accountID)

	return c.JSON(http.StatusOK, response)
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
	return global.BraintreePaymentFunction.Invoke(&invokeInput)
}
