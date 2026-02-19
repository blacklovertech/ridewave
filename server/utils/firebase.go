package utils

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
	"os"

	"go.uber.org/zap"
)

// FCM HTTP API (Legacy) — simple and dependency-free
// Uses the server key from Firebase Console > Project Settings > Cloud Messaging

type FCMNotification struct {
	Title string `json:"title"`
	Body  string `json:"body"`
	Sound string `json:"sound,omitempty"`
}

type FCMData map[string]string

type FCMMessage struct {
	To           string           `json:"to,omitempty"`
	Notification *FCMNotification `json:"notification,omitempty"`
	Data         FCMData          `json:"data,omitempty"`
	Priority     string           `json:"priority,omitempty"`
}

type FCMMulticastMessage struct {
	RegistrationIDs []string         `json:"registration_ids"`
	Notification    *FCMNotification `json:"notification,omitempty"`
	Data            FCMData          `json:"data,omitempty"`
	Priority        string           `json:"priority,omitempty"`
}

// SendPushNotification sends a push notification to a single device token
func SendPushNotification(token string, title, body string, data FCMData) error {
	serverKey := os.Getenv("FCM_SERVER_KEY")
	if serverKey == "" {
		Logger.Warn("FCM_SERVER_KEY not set, skipping push notification")
		return nil
	}
	if token == "" {
		return nil
	}

	msg := FCMMessage{
		To: token,
		Notification: &FCMNotification{
			Title: title,
			Body:  body,
			Sound: "default",
		},
		Data:     data,
		Priority: "high",
	}

	return sendFCM(serverKey, msg)
}

// SendPushToMultiple sends push notifications to multiple device tokens (max 1000)
func SendPushToMultiple(tokens []string, title, body string, data FCMData) error {
	serverKey := os.Getenv("FCM_SERVER_KEY")
	if serverKey == "" {
		Logger.Warn("FCM_SERVER_KEY not set, skipping push notifications")
		return nil
	}
	if len(tokens) == 0 {
		return nil
	}

	msg := FCMMulticastMessage{
		RegistrationIDs: tokens,
		Notification: &FCMNotification{
			Title: title,
			Body:  body,
			Sound: "default",
		},
		Data:     data,
		Priority: "high",
	}

	return sendFCM(serverKey, msg)
}

func sendFCM(serverKey string, payload interface{}) error {
	jsonData, err := json.Marshal(payload)
	if err != nil {
		return err
	}

	req, err := http.NewRequest("POST", "https://fcm.googleapis.com/fcm/send", bytes.NewBuffer(jsonData))
	if err != nil {
		return err
	}
	req.Header.Set("Authorization", "key="+serverKey)
	req.Header.Set("Content-Type", "application/json")

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		Logger.Error("FCM request failed", zap.Error(err))
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode >= 400 {
		Logger.Error("FCM error", zap.Int("status", resp.StatusCode))
		return fmt.Errorf("FCM error: %s", resp.Status)
	}

	Logger.Info("FCM notification sent", zap.Int("status", resp.StatusCode))
	return nil
}
