package android

import (
	"context"
	"encoding/base64"
	"os"

	"github.com/awa/go-iap/playstore"
	"google.golang.org/api/androidpublisher/v3"
)

//GetSubscriptionInformation is to fetch subscription information from google server
func GetSubscriptionInformation(packageName string, subscriptionID string, token string) (*androidpublisher.SubscriptionPurchase, error) {
	ctx := context.Background()
	client, err := getPlaystoreClient()

	if err != nil {
		return nil, err
	}

	return client.VerifySubscription(ctx, packageName, subscriptionID, token)
}

func getPlaystoreClient() (*playstore.Client, error) {
	jsonKeyBase64 := os.Getenv("GOOGLE_PLAYSTORE_JSON_KEY")
	jsonKeyStr, err := base64.StdEncoding.DecodeString(jsonKeyBase64)
	if err != nil {
		return nil, err
	}
	jsonKey := []byte(jsonKeyStr)

	return playstore.New(jsonKey)
}
