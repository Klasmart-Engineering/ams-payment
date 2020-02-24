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

	// SettleTransactionByReceipt updates the transaction to settled
	SettleTransactionByTransactionCode(transactionCode *TransactionCode) error

	// ReverseTransactionByReceipt updates the transaction to reversed and remove transaction related accesses
	ReverseTransactionByTransactionCode(transactionCode *TransactionCode) error
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

// SettleTransactionByReceipt updates the transaction to settled
func (transactionService *TransactionStandardService) SettleTransactionByTransactionCode(transactionCode *TransactionCode) error {
	transactionID, err := buildTransactionIDFromTransactionCode(transactionCode)
	if err != nil {
		return err
	}

	accTransactionInfo, err := transactionService.AccountDatabase.GetAccountTransactionInfoByTransactionID(transactionID)
	if err != nil {
		return err
	} else if accTransactionInfo == nil {
		return errors.New(apierrors.ErrorPaymentTransactionNotFound.String())
	}

	return transactionService.AccountDatabase.UpdateAccountTransaction(&accountdatabase.UpdateAccountTransactionInfo{
		AccountID:     accTransactionInfo.AccountID,
		TransactionID: accTransactionInfo.TransactionID,
		State:         transactions.Settled,
	})
}

// ReverseTransactionByReceipt updates the transaction to reversed and remove transaction related accesses
func (transactionService *TransactionStandardService) ReverseTransactionByTransactionCode(transactionCode *TransactionCode) error {
	transactionID, err := buildTransactionIDFromTransactionCode(transactionCode)
	if err != nil {
		return err
	}

	accTransactionInfo, err := transactionService.AccountDatabase.GetAccountTransactionInfoByTransactionID(transactionID)
	if err != nil {
		return err
	} else if accTransactionInfo == nil {
		return errors.New(apierrors.ErrorPaymentTransactionNotFound.String())
	}

	// Update the state of the transaction
	err = transactionService.AccountDatabase.UpdateAccountTransaction(&accountdatabase.UpdateAccountTransactionInfo{
		AccountID:        accTransactionInfo.AccountID,
		TransactionID:    accTransactionInfo.TransactionID,
		State:            transactions.Reversed,
		CancellationDate: accTransactionInfo.CancellationDate,
	})
	if err != nil {
		return err
	}

	// Delete the passes accesses provided by the transaction, if any
	if len(accTransactionInfo.Passes) > 0 {
		passIDs := make([]string, 0, len(accTransactionInfo.Passes))
		for passID, _ := range accTransactionInfo.Passes {
			passIDs = append(passIDs, passID)
		}
		err = transactionService.PassAccessService.DeletePassAccesses(accTransactionInfo.AccountID, passIDs)
		if err != nil {
			return err
		}
	}

	// Delete the products accesses provided by the transaction, if any
	if len(accTransactionInfo.Products) > 0 {
		productIDs := make([]string, 0, len(accTransactionInfo.Products))
		for productID, _ := range accTransactionInfo.Products {
			productIDs = append(productIDs, productID)
		}
		err = transactionService.ProductAccessService.DeleteProductAccesses(accTransactionInfo.AccountID, productIDs)
		if err != nil {
			return err
		}
	}
	return nil
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
