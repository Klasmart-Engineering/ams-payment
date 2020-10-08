package iap

type PlayStoreReceiptPayload struct {
	JSON      string `json:"json"`
	Signature string `json:"signature"`
}

type PlayStoreReceiptJSON struct {
	PackageName string `json:"packageName"`
	ProductID   string `json:"productId"`
}

type PlayStoreReceipt struct {
	Store         string `json:"GooglePlay"`
	Payload       string `json:"Payload"`
	TransactionID string `json:"TransactionID"`
}
