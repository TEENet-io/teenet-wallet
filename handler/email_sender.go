// Copyright (C) 2026 TEENet Technology (Hong Kong) Limited. All rights reserved.
// SPDX-License-Identifier: GPL-3.0-only

package handler

import (
	"fmt"
	"log/slog"
	"net/smtp"
	"strings"
)

// EmailSender abstracts sending a verification email so tests can substitute
// a fake. Implementations must be safe for concurrent use.
type EmailSender interface {
	// Send delivers a verification email containing `code` to `to`.
	// `to` is already lowercase + trimmed.
	Send(to, code string) error
	// Mode returns "smtp", "mock", or "disabled" for startup logging.
	Mode() string
}

// SmtpEmailSender sends real email via net/smtp.
type SmtpEmailSender struct {
	Host     string
	Port     string
	Username string
	Password string
	From     string
}

func (s *SmtpEmailSender) Mode() string { return "smtp" }

func (s *SmtpEmailSender) Send(to, code string) error {
	addr := s.Host + ":" + s.Port
	auth := smtp.PlainAuth("", s.Username, s.Password, s.Host)

	subject := "Your TEENet Wallet verification code"
	body := buildVerificationEmailHTML(code)
	from := s.From
	if from == "" {
		from = s.Username
	}

	headers := []string{
		"From: " + from,
		"To: " + to,
		"Subject: " + subject,
		"MIME-Version: 1.0",
		"Content-Type: text/html; charset=UTF-8",
	}
	msg := strings.Join(headers, "\r\n") + "\r\n\r\n" + body

	if err := smtp.SendMail(addr, auth, s.Username, []string{to}, []byte(msg)); err != nil {
		return fmt.Errorf("smtp send: %w", err)
	}
	return nil
}

// MockEmailSender logs codes to stdout instead of sending them.
// Used for local development when SMTP_HOST is not configured.
type MockEmailSender struct{}

func (m *MockEmailSender) Mode() string { return "mock" }

func (m *MockEmailSender) Send(to, code string) error {
	slog.Info("MOCK EMAIL: verification code", "to", to, "code", code)
	return nil
}
