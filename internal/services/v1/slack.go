package v1

import (
	"fmt"
	"os"

	"github.com/multiplay/go-slack/chat"
	"github.com/multiplay/go-slack/webhook"
)

// SlackConfig json configuration
type SlackConfig struct {
	PaymentChannel string `json:"payment_channel" env:"PAYMENT_CHANNEL_SLACK_HOOK_URL"`
}

// SlackMessageService is a service sending messages to a Slack channel
type SlackMessageService struct {
	WebHookURL string
}

// SendMessage send a message to channel
func (slackMessageService *SlackMessageService) SendMessage(message string) {
	c := webhook.New(slackMessageService.WebHookURL)
	m := &chat.Message{Text: message}
	m.Send(c)
}

// SendMessageFormat send a format message to channel
func (slackMessageService *SlackMessageService) SendMessageFormat(format string, args ...interface{}) {
	c := webhook.New(slackMessageService.WebHookURL)

	text := fmt.Sprintf("[%s] ", os.Getenv("SERVER_STAGE"))
	text += fmt.Sprintf(format, args...)

	m := &chat.Message{Text: text}
	m.Send(c)
}
