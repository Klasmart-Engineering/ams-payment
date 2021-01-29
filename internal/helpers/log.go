package utils

import (
	"encoding/json"
	"fmt"
	"os"

	"bitbucket.org/calmisland/payment-lambda-funcs/internal/global"
	log "github.com/sirupsen/logrus"
)

func LogFormat(contextLogger *log.Entry, format string, args ...interface{}) {
	contextLogger.Infof(format, args...)

	jsonMap := contextLogger.Data
	jsonMap["env"] = os.Getenv("SERVER_STAGE")
	jsonMap["message"] = fmt.Sprintf(format, args...)

	jsonObj, err := json.Marshal(jsonMap)

	if err != nil {
		contextLogger.Errorf("JSON marshalling process failure for a slack message")
	}

	global.PaymentSlackMessageService.SendMessage(string(jsonObj))
}
