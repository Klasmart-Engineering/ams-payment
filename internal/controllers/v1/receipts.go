package v1

import (
	"net/http"

	"bitbucket.org/calmisland/go-server-account/transactions"
	"bitbucket.org/calmisland/go-server-auth/authmiddlewares"
	"bitbucket.org/calmisland/go-server-product/passes"
	"bitbucket.org/calmisland/go-server-utils/timeutils"
	"bitbucket.org/calmisland/payment-lambda-funcs/internal/global"
	"github.com/labstack/echo/v4"
)

type getTransactionsResponse struct {
	Transactions []*getTransactionResponse `json:"transactions"`
	// TransactionsCount int32 `json:"transactions"`
}

type getTransactionResponse struct {
	TransactionID    string                        `json:"transactionId"`
	PassList         []*transactionPassResponse    `json:"passList"`
	ProductList      []*transactionProductResponse `json:"productList"`
	State            transactions.State            `json:"state"`
	CancellationDate timeutils.EpochTimeMS         `json:"cancellationDate"`
	CreatedDate      timeutils.EpochTimeMS         `json:"createdDate"`
	UpdatedDate      timeutils.EpochTimeMS         `json:"updatedDate"`
}

type transactionPassResponse struct {
	PassID    string                `json:"passId"`
	Price     string                `json:"price"`
	Currency  passes.Currency       `json:"currency"`
	StartDate timeutils.EpochTimeMS `json:"startDate"`
	Duration  passes.DurationDays   `json:"duration"`
}

type transactionProductResponse struct {
	ProductID string                `json:"productId"`
	StartDate timeutils.EpochTimeMS `json:"startDate"`
	Duration  passes.DurationDays   `json:"duration"`
}

func HandleGetReceipts(c echo.Context) error {
	cc := c.(*authmiddlewares.AuthContext)
	accountID := cc.Session.Data.AccountID
	transactionVOList, err := global.TransactionService.GetTransactionHistory(accountID)
	if err != nil {
		return err
	}

	transactions := make([]*getTransactionResponse, len(transactionVOList))
	for i, transactionVO := range transactionVOList {
		passes := make([]*transactionPassResponse, len(transactionVO.PassList))
		for j, pass := range transactionVO.PassList {
			priceStr, err := pass.Price.ToString(pass.Currency)
			if err != nil {
				return err
			}
			passes[j] = &transactionPassResponse{
				PassID:    pass.PassID,
				Price:     priceStr,
				Currency:  pass.Currency,
				StartDate: pass.StartDate,
				Duration:  pass.Duration,
			}
		}
		products := make([]*transactionProductResponse, len(transactionVO.ProductList))
		for k, product := range transactionVO.ProductList {
			products[k] = &transactionProductResponse{
				ProductID: product.ProductID,
				StartDate: product.StartDate,
				Duration:  product.Duration,
			}
		}
		transactions[i] = &getTransactionResponse{
			TransactionID:    transactionVO.TransactionID,
			PassList:         passes,
			ProductList:      products,
			State:            transactionVO.State,
			CancellationDate: transactionVO.CancellationDate,
			CreatedDate:      transactionVO.CreatedDate,
			UpdatedDate:      transactionVO.UpdatedDate,
		}
	}

	response := &getTransactionsResponse{
		Transactions: transactions,
	}
	return c.JSON(http.StatusOK, response)
}
