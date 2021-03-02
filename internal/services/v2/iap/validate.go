package iap

import (
	"errors"

	"bitbucket.org/calmisland/go-server-product/passes"
	"bitbucket.org/calmisland/go-server-product/productaccessservice"
	"bitbucket.org/calmisland/go-server-product/storeproducts"
	"bitbucket.org/calmisland/go-server-product/storeproductservice"
	"bitbucket.org/calmisland/go-server-utils/timeutils"
	calmerr "github.com/calmisland/go-errors"
)

var (
	ErrValidationNotFoundStoreProduct    = errors.New("cannot find store product")
	ErrValidationTranscationAlreadyExist = errors.New("transaction already exist")
)

type ValidationTranscationAlreadyProcessedByAnotherAccountError struct {
	AnotherAccountID string
}

func (e ValidationTranscationAlreadyProcessedByAnotherAccountError) Error() string {
	return "transaction already processed by another account"
}

// ValidateTransaction is to validate databases related to a payment transaction, and return StoreProductVO
func (transactionService *TransactionStandardService) ValidateTransaction(accountID string, storeProductID string, transactionCode TransactionCode) ([]*storeproductservice.StoreProductVO, error) {
	storeProducts, err := transactionService.StoreProductService.GetStoreProductVOListByStoreProductID(storeProductID)
	if err != nil {
		return nil, err
	} else if len(storeProducts) == 0 {
		return nil, ErrValidationNotFoundStoreProduct
	}

	// Validating transaction
	transaction, err := transactionService.GetTransactionByTransactionCode(&transactionCode)
	if err != nil {
		return nil, err
	} else if transaction != nil {
		if transaction.AccountID == accountID {
			return nil, ErrValidationTranscationAlreadyExist
		} else {
			return nil, &ValidationTranscationAlreadyProcessedByAnotherAccountError{AnotherAccountID: transaction.AccountID}
		}
	}

	return storeProducts, nil
}

func (transactionService *TransactionStandardService) RegisterTransaction(accountID string, storeProductID string, transactionCode TransactionCode, storeProducts []*storeproductservice.StoreProductVO, startTimeMillis int64, endTimeMills int64) error {
	productType := storeproducts.StoreProductTypeDefault
	passItems := []*PassItem{}
	productItems := []*productaccessservice.ProductAccessVOItem{}

	for _, product := range storeProducts {
		if productType == storeproducts.StoreProductTypeDefault {
			productType = product.Type
		} else if productType != product.Type {
			return calmerr.Errorf("Expected product type [%d] but found [%d] for store product ID: %s", productType, product.Type, storeProductID)
		}

		if product.Type == storeproducts.StoreProductTypePass {
			passInfo, err := transactionService.PassService.GetPassVOByPassID(product.ItemID)

			if err != nil {
				return err
			} else if passInfo == nil {
				return calmerr.Errorf("Unable to retrieve information about pass [%s] when processing IAP receipt", product.ItemID)
			}

			expiredTime := timeutils.EpochTimeMS(endTimeMills)

			passItems = append(passItems, &PassItem{
				PassID:        passInfo.PassID,
				Price:         passInfo.Price,
				Currency:      passInfo.Currency,
				StartDate:     timeutils.EpochTimeMS(startTimeMillis),
				ExpiresDateMS: expiredTime,
			})
		} else if product.Type == storeproducts.StoreProductTypeProduct {
			productItems = append(productItems, &productaccessservice.ProductAccessVOItem{
				ProductID:  product.ItemID,
				StartDate:  timeutils.EpochTimeMS(startTimeMillis),
				Duration:   passes.DurationDays((endTimeMills - startTimeMillis) / 1000 / 60 / 24),
				DurationMS: passes.DurationMilliseconds(endTimeMills - startTimeMillis),
			})
		}
	}

	if productType == storeproducts.StoreProductTypeProduct {
		err := transactionService.SaveTransactionUnlockProducts(accountID, storeProductID, &transactionCode, productItems)
		if err != nil {
			return err
		}
	} else if productType == storeproducts.StoreProductTypePass {
		err := transactionService.SaveTransactionUnlockPasses(accountID, storeProductID, &transactionCode, passItems)
		if err != nil {
			return err
		}
	}

	return nil
}
