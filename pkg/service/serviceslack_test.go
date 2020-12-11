package services_test

import (
	"testing"

	"bitbucket.org/calmisland/go-server-configs/configs"
	"bitbucket.org/calmisland/payment-lambda-funcs/pkg/global"
	"github.com/Jeffail/gabs/v2"
)

func TestSlackMessage(t *testing.T) {
	configs.UpdateConfigDirectoryPath("../../configs")
	global.SetupSlackMessageService()

	jsonObj := gabs.New()
	// or gabs.Wrap(jsonObject) to work on an existing map[string]interface{}

	jsonObj.Set("test", "accountId")

	global.PaymentSlackMessageService.SendMessage(jsonObj.String())
	global.PaymentSlackMessageService.SendMessageFormat("with Env %d", 100)
}
