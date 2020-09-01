package services_test

import (
	"testing"

	"bitbucket.org/calmisland/go-server-configs/configs"
	"bitbucket.org/calmisland/payment-lambda-funcs/src/globals"
	"bitbucket.org/calmisland/payment-lambda-funcs/src/setup/globalsetup"
	"github.com/Jeffail/gabs/v2"
)

func TestSlackMessage(t *testing.T) {
	configs.UpdateConfigDirectoryPath("../../configs")
	globalsetup.SetupSlackMessageService()

	jsonObj := gabs.New()
	// or gabs.Wrap(jsonObject) to work on an existing map[string]interface{}

	jsonObj.Set("test", "accountId")

	globals.PaymentSlackMessageService.SendMessage(jsonObj.String())
}
