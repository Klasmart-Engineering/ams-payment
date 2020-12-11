package iap

import (
	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/aws/session"
	"github.com/guregu/dynamo"
)

// InAppPlatformIOS is
type InAppPlatformIOS struct {
	BundelID     string `dynamo:"bundleId"`
	SharedSecret string `dynamo:"sharedSecret"`
}

// GetIosList to fetch every iap information for apple app store
func GetIosList() ([]InAppPlatformIOS, error) {
	db := dynamo.New(session.New(), &aws.Config{Region: aws.String("ap-northeast-1")})
	table := db.Table("iap_platform_ios")

	var results []InAppPlatformIOS
	err := table.Scan().All(&results)

	if err != nil {
		return nil, err
	}

	return results, nil
}
