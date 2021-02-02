package v1

import (
	"encoding/json"
	"errors"
	"fmt"
	"net/http"

	"github.com/labstack/echo/v4"
	log "github.com/sirupsen/logrus"

	"bitbucket.org/calmisland/go-server-account/transactions"
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
)

type paypalPaymentRequestBody struct {
	OrderID     string `json:"orderId"`
	ProductCode string `json:"productCode"`
}

func HandlePayPalPayment(c echo.Context) error {
	accountID := helpers.GetAccountID(c)

	reqBody := new(paypalPaymentRequestBody)
	err := c.Bind(reqBody)

	if err != nil {
		return apirequests.EchoSetClientError(c, apierrors.ErrorBadRequestBody)
	}

	contextLogger := log.WithFields(log.Fields{
		"paymentMethod": "Paypal",
		"accountID":     accountID,
		"orderId":       reqBody.OrderID,
		"productCode":   reqBody.ProductCode,
	})

	contextLogger.Info(reqBody)

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
		contextLogger.WithError(err)
		logFormat(contextLogger, "Error on getting a pass price by %s", passVO.Currency)
		return helpers.HandleInternalError(c, err)
	}
	response, err := makePaypalPayment(reqBody.OrderID)
	if err != nil {
		contextLogger.WithError(err)
		logFormat(contextLogger, "Error on making Paypal Payment")
		return helpers.HandleInternalError(c, err)
	}
	if !response.Success || passPrice != response.Value {
		logFormat(contextLogger, "Failed to make a Paypal Payment")
		return apirequests.EchoSetClientError(c, apierrors.ErrorBadRequestBody)
	}
	transactionCode := services.TransactionCode{
		Store: transactions.PayPal,
		ID:    reqBody.OrderID,
	}
	err = global.TransactionService.SaveTransactionUnlockPasses(accountID, &transactionCode, []*services.PassItem{item})
	if err != nil {
		contextLogger.WithError(err)
		logFormat(contextLogger, "Error on SaveTransactionUnlockPasses")
		return helpers.HandleInternalError(c, err)
	}

	// Send an email once a pass is unlocked
	accountInfo, err := global.AccountDatabase.GetAccountInfo(accountID)
	if err != nil {
		contextLogger.WithError(err)
		logFormat(contextLogger, "Error on GetAccountInfo")
		return helpers.HandleInternalError(c, err)
	} else if accountInfo == nil {
		logFormat(contextLogger, "Nil returned from GetAccountInfo")
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
		contextLogger.WithError(err)
		logFormat(contextLogger, "Error on EnqueueMessage to send an email")
		return helpers.HandleInternalError(c, err)
	}

	logFormat(contextLogger, "[PAYPALPAYMENT] A payment for the pass [%s] by account [%s] succeeded\n", passVO.PassID, accountID)

	return c.JSON(http.StatusOK, response)
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
	return global.PayPalPaymentFunction.Invoke(&invokeInput)
}
