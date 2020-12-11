package android

import (
	"context"

	"google.golang.org/api/androidpublisher/v3"
)

//GetSubscriptionInformation is to fetch subscription information from google server
func GetSubscriptionInformation(packageName string, subscriptionID string, token string) (*androidpublisher.SubscriptionPurchase, error) {
	ctx := context.Background()
	androidpublisherService, err := androidpublisher.NewService(ctx)

	if err != nil {
		return nil, err
	}

	purchasesSubscriptionsService := androidpublisher.NewPurchasesSubscriptionsService(androidpublisherService)

	purchasesGetCall := purchasesSubscriptionsService.Get(packageName, subscriptionID, token)
	result, err := purchasesGetCall.Do()

	return result, err
}
