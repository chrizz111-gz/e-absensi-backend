package utils

import (
	"fmt"
	"log"
	"net"
	"net/smtp"
	"os"
	"strconv"
	"strings"
)

// EmailSender adalah abstraksi pengiriman email OTP. Implementasi produksi
// memakai SMTP; mode dev (SMTP_HOST kosong) mencatat OTP ke konsol.
type EmailSender interface {
	SendOTP(to, name, code, purpose string) error
}

// NewEmailSender membuat EmailSender dari konfigurasi environment. Bila
// SMTP_HOST kosong, dipakai ConsoleEmailSender (memudahkan pengujian lifecycle
// OTP tanpa kredensial).
func NewEmailSender() EmailSender {
	host := strings.TrimSpace(os.Getenv("SMTP_HOST"))
	if host == "" {
		return &ConsoleEmailSender{}
	}

	port, err := strconv.Atoi(envStr("SMTP_PORT", "2525"))
	if err != nil {
		port = 2525
	}

	return &SMTPEmailSender{
		Host: host,
		Port: port,
		User: os.Getenv("SMTP_USER"),
		Pass: os.Getenv("SMTP_PASS"),
		From: envStr("SMTP_FROM", "E-Absensi <no-reply@e-absensi.local>"),
	}
}

// ConsoleEmailSender mencetak OTP ke log (mode development).
type ConsoleEmailSender struct{}

// SendOTP mencatat OTP ke konsol.
func (s *ConsoleEmailSender) SendOTP(to, name, code, purpose string) error {
	log.Printf("[DEV-EMAIL] OTP purpose=%s untuk %s <%s> => %s", purpose, name, to, code)
	return nil
}

// SMTPEmailSender mengirim email via SMTP (mis. Mailtrap untuk staging/dev).
type SMTPEmailSender struct {
	Host string
	Port int
	User string
	Pass string
	From string
}

// SendOTP mengirim email berisi kode OTP.
func (s *SMTPEmailSender) SendOTP(to, name, code, purpose string) error {
	subject := "Kode OTP E-Absensi"
	if purpose == "reset_password" {
		subject = "Kode Reset Password E-Absensi"
	}

	body := fmt.Sprintf(
		"<p>Halo %s,</p><p>Kode OTP Anda: <b>%s</b></p><p>Kode berlaku 5 menit. Jangan bagikan kode ini kepada siapa pun.</p>",
		name, code,
	)

	message := buildMessage(s.From, to, subject, body)
	addr := net.JoinHostPort(s.Host, strconv.Itoa(s.Port))
	var auth smtp.Auth
	if s.User != "" {
		auth = smtp.PlainAuth("", s.User, s.Pass, s.Host)
	}

	return smtp.SendMail(addr, auth, extractAddress(s.From), []string{to}, []byte(message))
}

// buildMessage menyusun email HTML sederhana.
func buildMessage(from, to, subject, htmlBody string) string {
	headers := []string{
		"From: " + from,
		"To: " + to,
		"Subject: " + subject,
		"MIME-Version: 1.0",
		"Content-Type: text/html; charset=\"UTF-8\"",
	}
	return strings.Join(headers, "\r\n") + "\r\n\r\n" + htmlBody
}

// extractAddress mengambil bagian alamat email dari format "Nama <email>".
func extractAddress(from string) string {
	if start := strings.Index(from, "<"); start >= 0 {
		if end := strings.Index(from, ">"); end > start {
			return strings.TrimSpace(from[start+1 : end])
		}
	}
	return strings.TrimSpace(from)
}

// envStr membaca environment dengan fallback.
func envStr(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}
