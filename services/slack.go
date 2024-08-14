package services

import (
	"bytes"
	"encoding/json"
	"net/http"
)

type SlackClient struct {
	WebhookURL string
}

func NewSlackClient(webhookURL string) *SlackClient {
	return &SlackClient{WebhookURL: webhookURL}
}

func (s *SlackClient) SendMessage(channel, message string) error {
	payload := map[string]string{
		"channel": channel,
		"text":    message,
	}
	body, _ := json.Marshal(payload)
	req, _ := http.NewRequest("POST", s.WebhookURL, bytes.NewBuffer(body))
	req.Header.Set("Content-Type", "application/json")

	client := &http.Client{}
	_, err := client.Do(req)
	return err
}
