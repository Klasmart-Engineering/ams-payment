package iap

import (
	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/aws/session"
	"github.com/guregu/dynamo"
)

// InAppPlatformAndroid is fsdfasdf
type InAppPlatformAndroid struct {
	ApplicationID string `dynamo:"applicationId"`
	PublicKey     string `dynamo:"publicKey"`
}

// GetAndroidList to fetch every iap information for google play store
func GetAndroidList() ([]InAppPlatformAndroid, error) {
	db := dynamo.New(session.New(), &aws.Config{Region: aws.String("ap-northeast-1")})
	table := db.Table("iap_platform_android")

	var results []InAppPlatformAndroid
	err := table.Scan().All(&results)

	if err != nil {
		return nil, err
	}

	return results, nil
}
