package handlers

import (
	"context"

	"bitbucket.org/calmisland/go-server-requests/apirequests"
	"bitbucket.org/calmisland/payment-lambda-funcs/src/globals"
	"bitbucket.org/calmisland/payment-lambda-funcs/src/services"
)

type getTransactionsResponse struct {
	Transactions []*services.Transaction `json:"transactions"`
	// TransactionsCount int32 `json:"transactions"`
}

func HandleGetReceipts(_ context.Context, req *apirequests.Request, resp *apirequests.Response) error {
	accountID := req.Session.Data.AccountID
	transactions, err := globals.TransactionService.GetTransactionHistory(accountID)

	if err != nil {
		return err
	}

	response := &getTransactionsResponse{
		Transactions: transactions,
	}
	resp.SetBody(response)
	return nil
}
