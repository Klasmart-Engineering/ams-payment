package services

import (
	"errors"
	"fmt"

	"bitbucket.org/calmisland/go-server-account/accountdatabase"
	"bitbucket.org/calmisland/go-server-account/transactions"
	"bitbucket.org/calmisland/go-server-product/passaccessservice"
	"bitbucket.org/calmisland/go-server-product/productaccessservice"
	"bitbucket.org/calmisland/go-server-utils/timeutils"
)

const transactionSeparator = "_"

// TransactionService TransactionService
var TransactionService ITransactionService

type ITransactionService interface {
	// GetTransaction return the transaction information based on an account and the associated receipt
	GetTransaction(accountID string, transactionCode *TransactionCode) (*Transaction, error)

	// GetTransactionByReceipt return the transaction information based on a receipt
	GetTransactionByTransactionCode(transactionCode *TransactionCode) (*Transaction, error)

	// SaveTransactionUnlockPasses save the transaction as pendingSettlement and add the associated passes accesses
	SaveTransactionUnlockPasses(accountID string, transactionCode *TransactionCode, items []*TransactionItem) error

	// SaveTransactionUnlockProducts save the transaction as pendingSettlement and add the associated products accesses
	SaveTransactionUnlockProducts(accountID string, transactionCode *TransactionCode, items []*TransactionItem) error

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
	PassList         []*TransactionItem
	ProductList      []*TransactionItem
	State            transactions.State
	CancellationDate timeutils.EpochTimeMS
	CreatedDate      timeutils.EpochTimeMS
	UpdatedDate      timeutils.EpochTimeMS
}

type TransactionItem struct {
	ItemID         string
	StartDate      timeutils.EpochTimeMS
	ExpirationDate timeutils.EpochTimeMS
}

type TransactionStandardService struct {
}

func init() {
	TransactionService = &TransactionStandardService{}
}

// GetTransaction return the transaction information based on an account and the associated receipt
func (transactionService *TransactionStandardService) GetTransaction(accountID string, transactionCode *TransactionCode) (*Transaction, error) {
	accountDB, err := accountdatabase.GetDatabase()
	if err != nil {
		return nil, err
	}
	transactionID, err := buildTransactionIDFromTransactionCode(transactionCode)
	if err != nil {
		return nil, err
	}
	accTransactionInfo, err := accountDB.GetAccountTransactionInfo(accountID, transactionID)
	if err != nil {
		return nil, err
	}
	return convertAccountTransactionInfoToTransaction(accTransactionInfo), nil
}

// GetTransactionByReceipt return the transaction information based on a receipt
func (transactionService *TransactionStandardService) GetTransactionByTransactionCode(transactionCode *TransactionCode) (*Transaction, error) {
	accountDB, err := accountdatabase.GetDatabase()
	if err != nil {
		return nil, err
	}
	transactionID, err := buildTransactionIDFromTransactionCode(transactionCode)
	if err != nil {
		return nil, err
	}
	accTransactionInfo, err := accountDB.GetAccountTransactionInfoByTransactionID(transactionID)
	if err != nil {
		return nil, err
	}
	return convertAccountTransactionInfoToTransaction(accTransactionInfo), nil
}

// SaveTransactionUnlockPasses save the transaction as pendingSettlement and add the associated passes accesses
func (transactionService *TransactionStandardService) SaveTransactionUnlockPasses(accountID string, transactionCode *TransactionCode, items []*TransactionItem) error {
	accountDB, err := accountdatabase.GetDatabase()
	if err != nil {
		return err
	}
	transactionID, err := buildTransactionIDFromTransactionCode(transactionCode)
	if err != nil {
		return err
	}
	err = accountDB.CreateAccountTransaction(&accountdatabase.CreateAccountTransactionInfo{
		AccountID:     accountID,
		TransactionID: transactionID,
		Passes:        convertTransactionItemListToItemMap(items),
		State:         transactions.PendingSettlement,
	})
	if err != nil {
		return err
	}

	existingPassAccessVOList, err := passaccessservice.PassAccessService.GetPassAccessVOListByAccountID(accountID)
	if err != nil {
		return err
	}
	passAccessVOList := make([]*passaccessservice.PassAccessVO, len(items))

	for _, item := range items {
		for _, passAccessVO := range existingPassAccessVOList {
			if item.ItemID == passAccessVO.PassID {
				item.ExpirationDate = computeNewExpirationTime(passAccessVO.ExpirationDate, item.StartDate, item.ExpirationDate)
			}
		}
	}

	// Retrieve the existing transaction IDs for a pass
	existingAccessTransactionIDsMap := make(map[string][]string)
	for _, passAccessVO := range existingPassAccessVOList {
		existingAccessTransactionIDsMap[passAccessVO.PassID] = passAccessVO.TransactionIDs
	}

	for i, item := range items {
		expirationDate, err := setExpirationDateByStore(transactionCode.Store, item.ExpirationDate)
		// If an unknown store is used, skip the permissions (but transaction is still saved since payment has been accepted)
		if err != nil {
			return err
		}
		passAccessVOList[i] = &passaccessservice.PassAccessVO{
			AccountID:      accountID,
			TransactionIDs: append(existingAccessTransactionIDsMap[item.ItemID], transactionID),
			PassID:         item.ItemID,
			ExpirationDate: expirationDate,
		}
	}
	return passaccessservice.PassAccessService.CreateOrUpdatePassAccessVOList(passAccessVOList)
}

// SaveTransactionUnlockProducts save the transaction as pendingSettlement and add the associated products accesses
func (transactionService *TransactionStandardService) SaveTransactionUnlockProducts(accountID string, transactionCode *TransactionCode, items []*TransactionItem) error {
	accountDB, err := accountdatabase.GetDatabase()
	if err != nil {
		return err
	}
	transactionID, err := buildTransactionIDFromTransactionCode(transactionCode)
	if err != nil {
		return err
	}
	err = accountDB.CreateAccountTransaction(&accountdatabase.CreateAccountTransactionInfo{
		AccountID:     accountID,
		TransactionID: transactionID,
		Products:      convertTransactionItemListToItemMap(items),
		State:         transactions.PendingSettlement,
	})
	if err != nil {
		return err
	}

	existingProductAccessVOList, err := productaccessservice.ProductAccessService.GetProductAccessVOListByAccountID(accountID)
	if err != nil {
		return err
	}
	productAccessVOList := make([]*productaccessservice.ProductAccessVO, len(items))

	for _, item := range items {
		for _, productAccessVO := range existingProductAccessVOList {
			if item.ItemID == productAccessVO.ProductID {
				item.ExpirationDate = computeNewExpirationTime(productAccessVO.ExpirationDate, item.StartDate, item.ExpirationDate)
			}
		}
	}

	// Retrieve the existing transaction IDs for a product
	existingAccessTransactionIDsMap := make(map[string][]string)
	for _, productAccessVO := range existingProductAccessVOList {
		existingAccessTransactionIDsMap[productAccessVO.ProductID] = productAccessVO.TransactionIDs
	}

	for i, item := range items {
		expirationDate, err := setExpirationDateByStore(transactionCode.Store, item.ExpirationDate)
		// If an unknown store is used, skip the permissions (but transaction is still saved since payment has been accepted)
		if err != nil {
			return err
		}
		productAccessVOList[i] = &productaccessservice.ProductAccessVO{
			AccountID:      accountID,
			TransactionIDs: append(existingAccessTransactionIDsMap[item.ItemID], transactionID),
			ProductID:      item.ItemID,
			ExpirationDate: expirationDate,
		}
	}
	return productaccessservice.ProductAccessService.CreateOrUpdateProductAccessVOList(productAccessVOList)
}

// SettleTransactionByReceipt updates the transaction to settled
func (transactionService *TransactionStandardService) SettleTransactionByTransactionCode(transactionCode *TransactionCode) error {
	accountDB, err := accountdatabase.GetDatabase()
	if err != nil {
		return err
	}
	transactionID, err := buildTransactionIDFromTransactionCode(transactionCode)
	if err != nil {
		return err
	}
	accTransactionInfo, err := accountDB.GetAccountTransactionInfoByTransactionID(transactionID)
	if err != nil {
		return err
	}
	return accountDB.UpdateAccountTransaction(&accountdatabase.UpdateAccountTransactionInfo{
		AccountID:     accTransactionInfo.AccountID,
		TransactionID: accTransactionInfo.TransactionID,
		State:         transactions.Settled,
	})
}

// ReverseTransactionByReceipt updates the transaction to reversed and remove transaction related accesses
func (transactionService *TransactionStandardService) ReverseTransactionByTransactionCode(transactionCode *TransactionCode) error {
	accountDB, err := accountdatabase.GetDatabase()
	if err != nil {
		return err
	}
	transactionID, err := buildTransactionIDFromTransactionCode(transactionCode)
	if err != nil {
		return err
	}
	accTransactionInfo, err := accountDB.GetAccountTransactionInfoByTransactionID(transactionID)
	if err != nil {
		return err
	}

	// Update the state of the transaction
	err = accountDB.UpdateAccountTransaction(&accountdatabase.UpdateAccountTransactionInfo{
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
		var passIDs []string
		for passID, _ := range accTransactionInfo.Passes {
			passIDs = append(passIDs, passID)
		}
		err = passaccessservice.PassAccessService.DeletePassAccesses(accTransactionInfo.AccountID, passIDs)
		if err != nil {
			return err
		}
	}

	// Delete the products accesses provided by the transaction, if any
	if len(accTransactionInfo.Products) > 0 {
		var productIDs []string
		for productID, _ := range accTransactionInfo.Products {
			productIDs = append(productIDs, productID)
		}
		err = productaccessservice.ProductAccessService.DeleteProductAccesses(accTransactionInfo.AccountID, productIDs)
		if err != nil {
			return err
		}
	}
	return nil
}

func buildTransactionIDFromTransactionCode(transactionCode *TransactionCode) (string, error) {
	switch transactionCode.Store {
	case transactions.GooglePlay, transactions.Apple, transactions.BrainTree:
		return fmt.Sprintf("%s%s%s", transactionCode.Store, transactionSeparator, transactionCode.ID), nil
	default:
		return "", errors.New("Unknown transaction store")
	}
}

func setExpirationDateByStore(store transactions.Store, expirationDate timeutils.EpochTimeMS) (timeutils.EpochTimeMS, error) {
	switch store {
	case transactions.GooglePlay, transactions.Apple, transactions.BrainTree:
		return expirationDate, nil
	default:
		return 0, errors.New("Unknown transaction store")
	}
}

func computeNewExpirationTime(oldExpirationDate, newStartDate, newExpirationDate timeutils.EpochTimeMS) timeutils.EpochTimeMS {
	deltaDuration := newExpirationDate.Time().Sub(newStartDate.Time())
	return oldExpirationDate.Add(deltaDuration)
}

func convertItemMapToTransactionItem(itemMap map[string]*accountdatabase.AccountTransactionItem) []*TransactionItem {
	transactionItemList := make([]*TransactionItem, len(itemMap))
	for key, value := range itemMap {
		transactionItemList = append(transactionItemList, &TransactionItem{
			ItemID:         key,
			StartDate:      value.StartDate,
			ExpirationDate: value.ExpirationDate,
		})
	}
	return transactionItemList
}

func convertTransactionItemListToItemMap(items []*TransactionItem) map[string]*accountdatabase.AccountTransactionItem {
	var itemMap map[string]*accountdatabase.AccountTransactionItem
	for _, item := range items {
		itemMap[item.ItemID] = &accountdatabase.AccountTransactionItem{
			StartDate:      item.StartDate,
			ExpirationDate: item.ExpirationDate,
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
	return &Transaction{
		AccountID:        accTransactionInfo.AccountID,
		TransactionID:    accTransactionInfo.TransactionID,
		PassList:         convertItemMapToTransactionItem(accTransactionInfo.Passes),
		ProductList:      convertItemMapToTransactionItem(accTransactionInfo.Products),
		State:            accTransactionInfo.State,
		CancellationDate: accTransactionInfo.CancellationDate,
		CreatedDate:      accTransactionInfo.CreatedDate,
		UpdatedDate:      accTransactionInfo.UpdatedDate,
	}
}
