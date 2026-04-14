// Copyright (C) 2026 TEENet Technology (Hong Kong) Limited. All rights reserved.
// SPDX-License-Identifier: GPL-3.0-only

package handler

import "fmt"

// buildVerificationEmailHTML renders the registration verification email.
// Style is intentionally close to the UMS verification template so users
// see a consistent visual identity across products.
func buildVerificationEmailHTML(code string) string {
	return fmt.Sprintf(`<!DOCTYPE html>
<html>
<head>
<meta charset="UTF-8">
<meta name="viewport" content="width=device-width, initial-scale=1.0">
<title>Verification Code</title>
<style>
body { font-family: Arial, sans-serif; line-height: 1.6; color: #333; }
.container { max-width: 600px; margin: 0 auto; padding: 20px; }
.header { background: #1890ff; color: white; padding: 20px; text-align: center; border-radius: 8px 8px 0 0; }
.content { background: #f9f9f9; padding: 30px; border-radius: 0 0 8px 8px; }
.code { background: #fff; border: 2px solid #1890ff; border-radius: 8px; padding: 20px; text-align: center; margin: 20px 0; }
.code-number { font-size: 32px; font-weight: bold; color: #1890ff; letter-spacing: 5px; }
.footer { text-align: center; margin-top: 20px; color: #666; font-size: 14px; }
.warning { background: #fff3cd; border: 1px solid #ffeaa7; padding: 15px; border-radius: 4px; margin: 15px 0; }
</style>
</head>
<body>
<div class="container">
  <div class="header">
    <h1>TEENet Wallet</h1>
    <p>Registration Verification</p>
  </div>
  <div class="content">
    <h2>Your Verification Code</h2>
    <p>Hello! You requested a verification code to register a new TEENet Wallet account.</p>
    <div class="code">
      <p>Your verification code is:</p>
      <div class="code-number">%s</div>
    </div>
    <div class="warning">
      <strong>Important:</strong>
      <ul style="margin: 10px 0; padding-left: 20px;">
        <li>This code will expire in <strong>10 minutes</strong></li>
        <li>Don't share this code with anyone</li>
        <li>If you didn't request this code, please ignore this email</li>
      </ul>
    </div>
  </div>
  <div class="footer">
    <p>This is an automated message, please do not reply to this email.</p>
  </div>
</div>
</body>
</html>`, code)
}
