package services

import (
	"fmt"

	"bitbucket.org/calmisland/go-server-account/accountdatabase"
	"bitbucket.org/calmisland/go-server-account/transactions"
	"bitbucket.org/calmisland/go-server-product/passaccessservice"
	"bitbucket.org/calmisland/go-server-product/passes"
	"bitbucket.org/calmisland/go-server-product/passservice"
	"bitbucket.org/calmisland/go-server-product/productaccessservice"
	"bitbucket.org/calmisland/go-server-requests/apierrors"
	"bitbucket.org/calmisland/go-server-utils/timeutils"
	"github.com/calmisland/go-errors"
)

const (
	transactionSeparator = "_"
)

type ITransactionService interface {
	// GetTransaction return the transaction information based on an account and the associated receipt
	GetTransaction(accountID string, transactionCode *TransactionCode) (*Transaction, error)

	// GetTransactionHistory return the a list of transactions for an account
	GetTransactionHistory(accountID string) ([]*Transaction, error)

	// GetTransactionByReceipt return the transaction information based on a receipt
	GetTransactionByTransactionCode(transactionCode *TransactionCode) (*Transaction, error)

	// SaveTransactionUnlockPasses save the transaction as pendingSettlement and add the associated passes accesses
	SaveTransactionUnlockPasses(accountID string, transactionCode *TransactionCode, items []*PassItem) error

	// SaveTransactionUnlockProducts save the transaction as pendingSettlement and add the associated products accesses
	SaveTransactionUnlockProducts(accountID string, transactionCode *TransactionCode, items []*productaccessservice.ProductAccessVOItem) error
}

type TransactionCode struct {
	Store transactions.Store
	ID    string
}

type Transaction struct {
	AccountID        string
	TransactionID    string
	TransactionCode  *TransactionCode
	PassList         []*PassItem
	ProductList      []*productaccessservice.ProductAccessVOItem
	State            transactions.State
	CancellationDate timeutils.EpochTimeMS
	CreatedDate      timeutils.EpochTimeMS
	UpdatedDate      timeutils.EpochTimeMS
}

type PassItem struct {
	PassID    string
	Price     passes.Price
	Currency  passes.Currency
	StartDate timeutils.EpochTimeMS
	Duration  passes.DurationDays
}

type TransactionStandardService struct {
	AccountDatabase      accountdatabase.Database
	PassService          *passservice.StandardPassService
	PassAccessService    *passaccessservice.StandardPassAccessService
	ProductAccessService *productaccessservice.StandardProductAccessService
}

// GetTransaction return the transaction information based on an account and the associated receipt
func (transactionService *TransactionStandardService) GetTransaction(accountID string, transactionCode *TransactionCode) (*Transaction, error) {
	transactionID, err := buildTransactionIDFromTransactionCode(transactionCode)
	if err != nil {
		return nil, err
	}

	accTransactionInfo, err := transactionService.AccountDatabase.GetAccountTransactionInfo(accountID, transactionID)
	if err != nil {
		return nil, err
	} else if accTransactionInfo == nil {
		return nil, nil
	}

	return convertAccountTransactionInfoToTransaction(accTransactionInfo), nil
}

// GetTransaction return the transaction information based on an account and the associated receipt
func (transactionService *TransactionStandardService) GetTransactionHistory(accountID string) ([]*Transaction, error) {
	accountTransactions, err := transactionService.AccountDatabase.GetAccountTransactionHistory(accountID)
	if err != nil {
		return nil, err
	}

	transactionHistory := make([]*Transaction, 0, len(accountTransactions))
	for _, accountTransaction := range accountTransactions {
		transactionHistory = append(transactionHistory, convertAccountTransactionInfoToTransaction(accountTransaction))
	}
	return transactionHistory, nil
}

// GetTransactionByReceipt return the transaction information based on a receipt
func (transactionService *TransactionStandardService) GetTransactionByTransactionCode(transactionCode *TransactionCode) (*Transaction, error) {
	transactionID, err := buildTransactionIDFromTransactionCode(transactionCode)
	if err != nil {
		return nil, err
	}

	accTransactionInfo, err := transactionService.AccountDatabase.GetAccountTransactionInfoByTransactionID(transactionID)
	if err != nil {
		return nil, err
	} else if accTransactionInfo == nil {
		return nil, nil
	}

	return convertAccountTransactionInfoToTransaction(accTransactionInfo), nil
}

// SaveTransactionUnlockPasses save the transaction as pendingSettlement and add the associated passes accesses
func (transactionService *TransactionStandardService) SaveTransactionUnlockPasses(accountID string, transactionCode *TransactionCode, items []*PassItem) error {
	transactionID, err := buildTransactionIDFromTransactionCode(transactionCode)
	if err != nil {
		return err
	}

	existingPassAccessVOList, err := transactionService.PassAccessService.GetPassAccessVOListByAccountID(accountID)
	if err != nil {
		return err
	}

	// Check if all pass are not already active
	passIds := make([]string, len(items))
	for i, item := range items {
		passIds[i] = item.PassID
		for _, passAccessVO := range existingPassAccessVOList {
			if item.PassID == passAccessVO.PassID {
				// Cumulate Pass forbidden
				return errors.New(apierrors.ErrorPaymentPassAccessAlreadyExist.String())
			}
		}
	}

	// Create the account transaction
	err = transactionService.AccountDatabase.CreateAccountTransaction(&accountdatabase.CreateAccountTransactionInfo{
		AccountID:     accountID,
		TransactionID: transactionID,
		Passes:        convertPassItemListToItemMap(items),
		State:         transactions.PendingSettlement,
	})
	if err != nil {
		return err
	}

	// Create Pass accesses
	passAccessVOList := make([]*passaccessservice.PassAccessVO, len(items))
	for i, item := range items {
		passAccessVOList[i] = &passaccessservice.PassAccessVO{
			AccountID:      accountID,
			PassID:         item.PassID,
			TransactionIDs: []string{transactionID},
			ExpirationDate: timeutils.ConvEpochTimeMS(item.StartDate.Time().AddDate(0, 0, int(item.Duration))),
			ActivationDate: item.StartDate,
		}
	}
	err = transactionService.PassAccessService.CreateOrUpdatePassAccessVOList(passAccessVOList)
	if err != nil {
		return err
	}

	// Save the passes one by one (rare case), for reusability purpose. Should be improved to save everything in 1 batch
	passVOList, err := transactionService.PassService.GetPassVOListByIds(passIds)
	if err != nil {
		return err
	}
	for _, item := range items {
		for _, passVO := range passVOList {
			if item.PassID == passVO.PassID {
				productItems := make([]*productaccessservice.ProductAccessVOItem, len(passVO.Products))
				for i, productID := range passVO.Products {
					productItems[i] = &productaccessservice.ProductAccessVOItem{
						ProductID: productID,
						StartDate: item.StartDate,
						Duration:  item.Duration,
					}
				}
				err := transactionService.ProductAccessService.CreateOrUpdateProductAccessVOListByTransaction(accountID, transactionID, productItems)
				if err != nil {
					return err
				}
			}
		}
	}

	return nil
}

// SaveTransactionUnlockProducts save the transaction as pendingSettlement and add the associated products accesses
func (transactionService *TransactionStandardService) SaveTransactionUnlockProducts(accountID string, transactionCode *TransactionCode, items []*productaccessservice.ProductAccessVOItem) error {
	transactionID, err := buildTransactionIDFromTransactionCode(transactionCode)
	if err != nil {
		return err
	}
	err = transactionService.AccountDatabase.CreateAccountTransaction(&accountdatabase.CreateAccountTransactionInfo{
		AccountID:     accountID,
		TransactionID: transactionID,
		Products:      convertProductAccessVOItemListToItemMap(items),
		State:         transactions.PendingSettlement,
	})
	if err != nil {
		return err
	}

	return transactionService.ProductAccessService.CreateOrUpdateProductAccessVOListByTransaction(accountID, transactionID, items)
}

func buildTransactionIDFromTransactionCode(transactionCode *TransactionCode) (string, error) {
	switch transactionCode.Store {
	case transactions.GooglePlay, transactions.Apple, transactions.BrainTree, transactions.PayPal:
		return fmt.Sprintf("%s%s%s", transactionCode.Store, transactionSeparator, transactionCode.ID), nil
	default:
		return "", errors.New(apierrors.ErrorPaymentUnknownTransactionStore.String())
	}
}

func computeNewExpirationTime(oldExpirationDate, newStartDate, newExpirationDate timeutils.EpochTimeMS) timeutils.EpochTimeMS {
	deltaDuration := newExpirationDate.Time().Sub(newStartDate.Time())
	return oldExpirationDate.Add(deltaDuration)
}

func convertItemMapToProductAccessVOItem(itemMap map[string]*accountdatabase.AccountTransactionItem) []*productaccessservice.ProductAccessVOItem {
	transactionItemList := make([]*productaccessservice.ProductAccessVOItem, 0, len(itemMap))
	for key, value := range itemMap {
		transactionItemList = append(transactionItemList, &productaccessservice.ProductAccessVOItem{
			ProductID: key,
			StartDate: value.StartDate,
			Duration:  passes.DurationDays(value.ExpirationDate.Time().Sub(value.StartDate.Time()).Hours() / 24),
		})
	}
	return transactionItemList
}

func convertItemMapToPassItem(itemMap map[string]*accountdatabase.AccountTransactionItem) []*PassItem {
	transactionItemList := make([]*PassItem, 0, len(itemMap))
	for key, value := range itemMap {
		transactionItemList = append(transactionItemList, &PassItem{
			PassID:    key,
			Price:     passes.Price(value.Price),
			Currency:  passes.Currency(value.Currency),
			StartDate: value.StartDate,
			Duration:  passes.DurationDays(value.ExpirationDate.Time().Sub(value.StartDate.Time()).Hours() / 24),
		})
	}
	return transactionItemList
}

func convertPassItemListToItemMap(items []*PassItem) map[string]*accountdatabase.AccountTransactionItem {
	itemMap := make(map[string]*accountdatabase.AccountTransactionItem)
	for _, item := range items {
		itemMap[item.PassID] = &accountdatabase.AccountTransactionItem{
			Price:          int32(item.Price),
			Currency:       string(item.Currency),
			StartDate:      item.StartDate,
			ExpirationDate: timeutils.ConvEpochTimeMS(item.StartDate.Time().AddDate(0, 0, int(item.Duration))),
		}
	}
	return itemMap
}

func convertProductAccessVOItemListToItemMap(items []*productaccessservice.ProductAccessVOItem) map[string]*accountdatabase.AccountTransactionItem {
	itemMap := make(map[string]*accountdatabase.AccountTransactionItem)
	for _, item := range items {
		itemMap[item.ProductID] = &accountdatabase.AccountTransactionItem{
			StartDate:      item.StartDate,
			ExpirationDate: timeutils.ConvEpochTimeMS(item.StartDate.Time().AddDate(0, 0, int(item.Duration))),
		}
	}
	return itemMap
}

func convertAccountTransactionInfoListToTransactionList(accountTransactionInfoList []*accountdatabase.AccountTransactionInfo) []*Transaction {
	transaction := make([]*Transaction, len(accountTransactionInfoList))
	for i, accTransactionInfo := range accountTransactionInfoList {
		transaction[i] = convertAccountTransactionInfoToTransaction(accTransactionInfo)
	}
	return transaction
}

func convertAccountTransactionInfoToTransaction(accTransactionInfo *accountdatabase.AccountTransactionInfo) *Transaction {
	if accTransactionInfo == nil {
		return nil
	}

	return &Transaction{
		AccountID:        accTransactionInfo.AccountID,
		TransactionID:    accTransactionInfo.TransactionID,
		PassList:         convertItemMapToPassItem(accTransactionInfo.Passes),
		ProductList:      convertItemMapToProductAccessVOItem(accTransactionInfo.Products),
		State:            accTransactionInfo.State,
		CancellationDate: accTransactionInfo.CancellationDate,
		CreatedDate:      accTransactionInfo.CreatedDate,
		UpdatedDate:      accTransactionInfo.UpdatedDate,
	}
}
