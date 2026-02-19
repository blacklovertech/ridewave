package utils

import (
	"bytes"
	"fmt"
	"net/http"
	"os"
)

// SendTwilioOTP sends an OTP via Twilio Verify
func SendTwilioOTP(phoneNumber string) error {
	accountSid := os.Getenv("TWILIO_ACCOUNT_SID")
	authToken := os.Getenv("TWILIO_AUTH_TOKEN")
	serviceSid := os.Getenv("TWILIO_SERVICE_SID")

	if accountSid == "" || authToken == "" || serviceSid == "" {
		return fmt.Errorf("twilio credentials not configured")
	}

	url := fmt.Sprintf("https://verify.twilio.com/v2/Services/%s/Verifications", serviceSid)
	data := fmt.Sprintf("To=%s&Channel=sms", phoneNumber)

	req, _ := http.NewRequest("POST", url, bytes.NewBufferString(data))
	req.SetBasicAuth(accountSid, authToken)
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")

	client := &http.Client{}
	resp, err := client.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode >= 400 {
		return fmt.Errorf("twilio error: %s", resp.Status)
	}
	return nil
}

// VerifyTwilioOTP verifies an OTP via Twilio Verify
func VerifyTwilioOTP(phoneNumber, code string) error {
	accountSid := os.Getenv("TWILIO_ACCOUNT_SID")
	authToken := os.Getenv("TWILIO_AUTH_TOKEN")
	serviceSid := os.Getenv("TWILIO_SERVICE_SID")

	if accountSid == "" || authToken == "" || serviceSid == "" {
		return fmt.Errorf("twilio credentials not configured")
	}

	url := fmt.Sprintf("https://verify.twilio.com/v2/Services/%s/VerificationChecks", serviceSid)
	data := fmt.Sprintf("To=%s&Code=%s", phoneNumber, code)

	req, _ := http.NewRequest("POST", url, bytes.NewBufferString(data))
	req.SetBasicAuth(accountSid, authToken)
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")

	client := &http.Client{}
	resp, err := client.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode >= 400 {
		return fmt.Errorf("twilio verification failed: %s", resp.Status)
	}
	return nil
}