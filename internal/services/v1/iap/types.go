package iap

type PlayStoreReceiptJSON struct {
	PackageName   string `json:"packageName"`
	ProductID     string `json:"productId"`
	OrderID       string `json:"orderId"`
	PurchaseToken string `json:"purchaseToken"`
}
