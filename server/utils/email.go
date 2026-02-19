package utils

import (
	"fmt"
	"net/smtp"
	"os"
)

func SendEmail(to []string, subject, body string) error {
	from := os.Getenv("SMTP_USER")
	password := os.Getenv("SMTP_PASS")
	host := os.Getenv("SMTP_HOST")
	port := os.Getenv("SMTP_PORT")

	auth := smtp.PlainAuth("", from, password, host)

	// Basic email headers
	headers := "MIME-Version: 1.0\r\n" +
		"Content-Type: text/html; charset=UTF-8\r\n" +
		fmt.Sprintf("From: RideWave <%s>\r\n", from) +
		fmt.Sprintf("To: %s\r\n", to[0]) +
		fmt.Sprintf("Subject: %s\r\n\r\n", subject)

	msg := []byte(headers + body)

	addr := fmt.Sprintf("%s:%s", host, port)
	
	if err := smtp.SendMail(addr, auth, from, to, msg); err != nil {
		return err
	}
	return nil
}
