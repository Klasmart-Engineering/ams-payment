package v1

import (
	"testing"

	"bitbucket.org/calmisland/go-server-configs/configs"
	"github.com/Jeffail/gabs/v2"
)

func TestSlackMessage(t *testing.T) {
	configs.UpdateConfigDirectoryPath("../../../configs")

	config := &SlackConfig{
		PaymentChannel: "https://hooks.slack.com/services/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",
	}

	serviceInstance := &SlackMessageService{WebHookURL: config.PaymentChannel}

	jsonObj := gabs.New()
	// or gabs.Wrap(jsonObject) to work on an existing map[string]interface{}

	jsonObj.Set("test", "accountId")

	serviceInstance.SendMessage(jsonObj.String())
	serviceInstance.SendMessageFormat("with Env %d", 100)
}
