package email

import (
	"fmt"
	"time"

	"flowwithlit/internal/company"
)

// BaseEmailTemplate wraps any content inside the global HTML email design
func BaseEmailTemplate(title string, contentHtml string) string {
	c := company.Get()
	siteURL := "https://flowwithlit.com"
	return fmt.Sprintf(`<!DOCTYPE html>
<html lang="en" xmlns="http://www.w3.org/1999/xhtml" xmlns:v="urn:schemas-microsoft-com:vml" xmlns:o="urn:schemas-microsoft-com:office:office">
<head>
    <meta charset="utf-8">
    <meta name="viewport" content="width=device-width, initial-scale=1.0">
    <meta http-equiv="X-UA-Compatible" content="IE=edge">
    <meta name="color-scheme" content="light dark">
    <meta name="supported-color-schemes" content="light dark">
    <title>%s</title>
    
    <!--[if gte mso 9]>
    <xml>
      <o:OfficeDocumentSettings>
        <o:AllowPNG/>
        <o:PixelsPerInch>96</o:PixelsPerInch>
      </o:OfficeDocumentSettings>
    </xml>
    <![endif]-->

    <style type="text/css">
        /* Client Resets */
        body, table, td, a { -webkit-text-size-adjust: 100%%; -ms-text-size-adjust: 100%%; }
        table, td { mso-table-lspace: 0pt; mso-table-rspace: 0pt; }
        img { -ms-interpolation-mode: bicubic; border: 0; height: auto; line-height: 100%%; outline: none; text-decoration: none; }
        
        /* Typography & Body Defaults */
        body { 
            margin: 0; 
            padding: 0; 
            width: 100%% !important; 
            background-color: #F9FAFC; /* Very light grey/blue background */
            font-family: -apple-system, BlinkMacSystemFont, 'Segoe UI', Roboto, Helvetica, Arial, sans-serif; 
        }

        /* Hover effect for the gold button */
        .btn-gold:hover {
            background-color: #e5a000 !important;
        }

        /* Responsive adjustments */
        @media screen and (max-width: 500px) {
            .container { width: 100%% !important; padding: 20px 15px !important; }
            .card { padding: 24px 20px !important; }
        }

        /* Navy Blue Dark Mode */
        @media (prefers-color-scheme: dark) {
            body, center { background-color: #020C1B !important; }
            .card { background-color: #0A192F !important; border: 1px solid #1a2c4e !important; }
            p, h2, span, strong { color: #E2E8F0 !important; }
            .quick-links a { color: #E2E8F0 !important; }
            .footer-text, .footer-text a { color: #94A3B8 !important; }
        }
    </style>
</head>
<body style="margin: 0; padding: 0; background-color: #F9FAFC;">
    <center style="width: 100%%; table-layout: fixed; background-color: #F9FAFC;">
        <table width="100%%" cellpadding="0" cellspacing="0" border="0">
            <tr>
                <td align="center" class="container" style="padding: 50px 20px;">
                    
                    <table width="100%%" max-width="500" cellpadding="0" cellspacing="0" border="0" style="max-width: 500px;">
                        <tr>
                            <td align="center" style="padding-bottom: 24px;">
                                <a href="%s" target="_blank" style="text-decoration: none;">
                                    <img src="%s/assets/images/full-logo.png" alt="%s" width="200" style="display: block; width: 200px; max-width: 100%%; height: auto; border: 0; font-family: sans-serif; font-size: 20px; font-weight: bold; color: #0A192F;">
                                </a>
                            </td>
                        </tr>
                    </table>

                    <!-- Main White Content Box -->
                    <table class="card" width="100%%" cellpadding="0" cellspacing="0" border="0" style="width: 100%%; background-color: #FFFFFF; border: 1px solid #EAECEF; border-radius: 4px; box-shadow: 0 2px 4px rgba(0,0,0,0.02);">
                        <tr>
                            <td style="padding: 40px; text-align: left;">
                                %s
                            </td>
                        </tr>
                    </table>

                    <table width="100%%" max-width="500" cellpadding="0" cellspacing="0" border="0" style="max-width: 500px;">
                        <tr>
                            <td align="center" style="padding-top: 24px;">
                                
                                <!-- Quick Links -->
                                <p class="quick-links" style="margin: 0 0 16px 0; font-size: 12px; color: #666666;">
                                    <a href="%s/help" style="color: #0A192F; text-decoration: none; font-weight: 600;">Help Center</a> &nbsp;&bull;&nbsp; 
                                    <a href="%s/privacy" style="color: #0A192F; text-decoration: none; font-weight: 600;">Privacy Policy</a> &nbsp;&bull;&nbsp; 
                                    <a href="%s/terms" style="color: #0A192F; text-decoration: none; font-weight: 600;">Terms of Service</a>
                                </p>

                                <!-- Company Address & General Info -->
                                <p class="footer-text" style="margin: 0 0 12px 0; font-size: 12px; line-height: 18px; color: #888888;">
                                    <strong>%s</strong><br>
                                    %s<br>
                                    <a href="mailto:%s" style="color: #888888; text-decoration: none;">%s</a>
                                </p>

                                <!-- Disclaimer & Unsubscribe -->
                                <p class="footer-text" style="margin: 0; font-size: 11px; line-height: 16px; color: #AAAAAA;">
                                    You are receiving this email because you signed up for a Flowwithlit account.<br>
                                    If you did not make this request, you can safely ignore this email.<br>
                                    <br>
                                    <a href="#" style="color: #888888; text-decoration: underline;">Unsubscribe</a> from future notifications.
                                </p>

                            </td>
                        </tr>
                    </table>

                </td>
            </tr>
        </table>
    </center>
</body>
</html>`, title, siteURL, siteURL, c.CompanyName, contentHtml, siteURL, siteURL, siteURL, c.LegalName, c.FormattedAddress(), c.SupportEmail, c.SupportEmail)
}

// GetWelcomeEmail returns the HTML for the welcome email
func GetWelcomeEmail(firstName string, frontendURL string) string {
	content := fmt.Sprintf(`
		<p style="margin: 0 0 20px 0; font-size: 15px; line-height: 24px; color: #4A4A4A;">
			Hello %s, 👋<br><br>
			We are thrilled to have you onboard. FlowWithLit is your premier digital wallet and neobanking gateway.<br><br>
			You can now manage your finances, send E-Transfers, and create virtual cards seamlessly.
		</p>
		<table width="100%%" cellpadding="0" cellspacing="0" border="0" style="margin-bottom: 20px; margin-top: 20px;">
			<tr>
				<td align="center">
					<a href="%s" class="btn-gold" style="display: block; width: 100%%; background-color: #F2A900; color: #0A192F; font-size: 15px; font-weight: 600; text-decoration: none; padding: 14px 0; border-radius: 4px; text-align: center;">
						Go to Dashboard
					</a>
				</td>
			</tr>
		</table>
		<p style="margin: 0; font-size: 14px; line-height: 22px; color: #999999;">
			If you have any questions, simply reply to this email. We're here to help!
		</p>
	`, firstName, frontendURL)

	return BaseEmailTemplate("Welcome to FlowWithLit", content)
}

// GetLoginAlertEmail returns the HTML for the new login alert
func GetLoginAlertEmail(firstName string, ipAddress string) string {
	currentTime := time.Now().Format("Jan 02, 2006 at 03:04 PM")
	
	content := fmt.Sprintf(`
		<h2 style="margin: 0 0 15px 0; font-size: 20px; color: #0A192F;">New Login Detected 🔒</h2>
		<p style="margin: 0 0 20px 0; font-size: 15px; line-height: 24px; color: #4A4A4A;">
			Hello %s,<br><br>
			We noticed a new login to your FlowWithLit account. If this was you, you can safely ignore this email.
		</p>
		<div style="background-color: #F9FAFC; padding: 20px; border-radius: 4px; border: 1px solid #EAECEF; margin-bottom: 25px;">
			<p style="margin: 0 0 10px 0; font-size: 14px; color: #0A192F;"><strong>Time:</strong> %s</p>
			<p style="margin: 0; font-size: 14px; color: #0A192F;"><strong>IP Address:</strong> %s</p>
		</div>
		<p style="margin: 0; font-size: 14px; line-height: 22px; color: #E53E3E; font-weight: 500;">
			If you did not authorize this login, please secure your account immediately.
		</p>
	`, firstName, currentTime, ipAddress)

	return BaseEmailTemplate("New Login Alert", content)
}

// GetPasswordResetEmail returns the HTML containing the 6-digit OTP for password recovery
func GetPasswordResetEmail(firstName string, otp string) string {
	content := fmt.Sprintf(`
		<h2 style="margin: 0 0 15px 0; font-size: 20px; color: #0A192F;">Password Reset 🔐</h2>
		<p style="margin: 0 0 20px 0; font-size: 15px; line-height: 24px; color: #4A4A4A;">
			Hello %s,<br><br>
			We received a request to reset your password. Use the verification code below to proceed.
		</p>
		<div style="background-color: #F9FAFC; padding: 25px; border-radius: 4px; border: 2px dashed #EAECEF; text-align: center; margin: 0 auto 25px auto; width: 90%%; box-sizing: border-box;">
			<span style="font-family: monospace; font-size: 32px; font-weight: bold; letter-spacing: 6px; color: #0A192F;">%s</span>
		</div>
		<p style="margin: 0; font-size: 14px; line-height: 22px; color: #4A4A4A;">
			This code expires in 15 minutes. If you did not request a password reset, please ignore this email.
		</p>
	`, firstName, otp)

	return BaseEmailTemplate("Password Reset", content)
}

// GetEmailVerificationEmail returns the HTML containing the 6-digit OTP for account activation
func GetEmailVerificationEmail(firstName string, otp string) string {
	content := fmt.Sprintf(`
		<h2 style="margin: 0 0 15px 0; font-size: 20px; color: #0A192F;">Verify Your Email ✅</h2>
		<p style="margin: 0 0 20px 0; font-size: 15px; line-height: 24px; color: #4A4A4A;">
			Hello %s,<br><br>
			Welcome to FlowWithLit! Please use the verification code below to activate your account.
		</p>
		<div style="background-color: #F9FAFC; padding: 25px; border-radius: 4px; border: 2px dashed #EAECEF; text-align: center; margin: 0 auto 25px auto; width: 90%%; box-sizing: border-box;">
			<span style="font-family: monospace; font-size: 32px; font-weight: bold; letter-spacing: 6px; color: #0A192F;">%s</span>
		</div>
		<p style="margin: 0; font-size: 14px; line-height: 22px; color: #4A4A4A;">
			This code expires in 15 minutes.
		</p>
	`, firstName, otp)

	return BaseEmailTemplate("Verify Your Email", content)
}

// GetFlowTagReceivedEmail creates a classic, professional email for receiving a FlowTag (supports non-registered recipients)
func GetFlowTagReceivedEmail(senderName string, senderEmail string, amount float64, currency string, claimURL string, expiresIn string) string {
	amountStr := fmt.Sprintf("%.2f", amount)

	symbol := currency
	switch currency {
	case "NGN":
		symbol = "₦"
	case "USD", "USDT":
		symbol = "$"
	case "EUR":
		symbol = "€"
	}

	content := fmt.Sprintf(`
		<h2 style="margin: 0 0 12px 0; font-size: 22px; color: #0A192F; font-weight: 600;">You have received a payment</h2>
		
		<p style="margin: 0 0 20px 0; font-size: 15px; line-height: 24px; color: #374151;">
			Hello,<br><br>
			<strong>%s</strong> (%s) has sent you funds through Flowwithlit.
		</p>

		<div style="background-color: #F8FAFC; border: 1px solid #E2E8F0; border-radius: 6px; padding: 22px; margin-bottom: 26px; text-align: center;">
			<div style="font-size: 12px; color: #64748B; margin-bottom: 2px; letter-spacing: 1px;">AMOUNT</div>
			<div style="font-size: 38px; font-weight: 700; color: #0A192F; line-height: 1.1;">%s%s</div>
		</div>

		<p style="margin: 0 0 18px 0; font-size: 15px; line-height: 24px; color: #374151;">
			The funds have been reserved for you. Please claim them into your Flowwithlit wallet using the link below.
		</p>

		<table width="100%%" cellpadding="0" cellspacing="0" border="0" style="margin-bottom: 22px;">
			<tr>
				<td align="center">
					<a href="%s" style="display: inline-block; background-color: #0A192F; color: #FFFFFF; font-size: 15px; font-weight: 600; text-decoration: none; padding: 14px 44px; border-radius: 4px; text-align: center;">
						Claim Funds
					</a>
				</td>
			</tr>
		</table>

		<p style="margin: 0 0 10px 0; font-size: 14px; color: #475569;">
			This claim link will expire in <strong>%s</strong>.
		</p>

		<p style="margin: 0; font-size: 13px; color: #64748B;">
			If you do not yet have a Flowwithlit account, sign up with this email address and then return to this link to claim your funds.
		</p>

		<p style="margin-top: 18px; font-size: 13px; color: #94A3B8;">
			If you were not expecting this payment, you may disregard this message.
		</p>
	`, senderName, senderEmail, symbol, amountStr, claimURL, expiresIn)

	return BaseEmailTemplate("Payment Received", content)
}

// GetSecureTransferReceivedEmail notifies recipient that funds are held — access key is shared separately by sender.
func GetSecureTransferReceivedEmail(senderName string, senderEmail string, amount float64, currency string, claimPageURL string, expiresIn string) string {
	amountStr := fmt.Sprintf("%.2f", amount)

	symbol := currency
	switch currency {
	case "NGN":
		symbol = "₦"
	case "USD", "USDT":
		symbol = "$"
	case "EUR":
		symbol = "€"
	}

	content := fmt.Sprintf(`
		<h2 style="margin: 0 0 12px 0; font-size: 22px; color: #0A192F; font-weight: 600;">E-Transfer waiting for you</h2>
		
		<p style="margin: 0 0 20px 0; font-size: 15px; line-height: 24px; color: #374151;">
			Hello,<br><br>
			<strong>%s</strong> (%s) has sent you money via Flowwithlit E-Transfer. The funds are locked in escrow until you claim them.
		</p>

		<div style="background-color: #F5F3FF; border: 1px solid #DDD6FE; border-radius: 6px; padding: 22px; margin-bottom: 26px; text-align: center;">
			<div style="font-size: 12px; color: #7C3AED; margin-bottom: 2px; letter-spacing: 1px;">AMOUNT HELD</div>
			<div style="font-size: 38px; font-weight: 700; color: #0A192F; line-height: 1.1;">%s%s</div>
		</div>

		<p style="margin: 0 0 18px 0; font-size: 15px; line-height: 24px; color: #374151;">
			<strong>Important:</strong> You will <em>not</em> receive the access key in this email. The sender must share it with you separately (text, WhatsApp, call, etc.). Once you have the key, visit the claim page below.
		</p>

		<table width="100%%" cellpadding="0" cellspacing="0" border="0" style="margin-bottom: 22px;">
			<tr>
				<td align="center">
					<a href="%s" style="display: inline-block; background-color: #7C3AED; color: #FFFFFF; font-size: 15px; font-weight: 600; text-decoration: none; padding: 14px 44px; border-radius: 4px; text-align: center;">
						Claim E-Transfer
					</a>
				</td>
			</tr>
		</table>

		<p style="margin: 0 0 10px 0; font-size: 14px; color: #475569;">
			This transfer expires in <strong>%s</strong> if not claimed.
		</p>

		<p style="margin: 0; font-size: 13px; color: #64748B;">
			If you do not yet have a Flowwithlit account, sign up with this email address, then use your access key on the claim page.
		</p>
	`, senderName, senderEmail, symbol, amountStr, claimPageURL, expiresIn)

	return BaseEmailTemplate("E-Transfer Received", content)
}

// GetAdminInviteEmail generates the invitation email for a new admin staff member
func GetAdminInviteEmail(recipientEmail string, role string, inviteLink string) string {
	content := fmt.Sprintf(`
		<h2 style="margin: 0 0 12px 0; font-size: 22px; color: #0A192F; font-weight: 600;">You've been invited to Flowwithlit Admin</h2>

		<p style="margin: 0 0 20px 0; font-size: 15px; line-height: 24px; color: #374151;">
			You have been invited to join the Flowwithlit admin dashboard as a <strong style="color: #ff5400;">%s</strong>.
		</p>

		<div style="background-color: #F8FAFC; border: 1px solid #E2E8F0; border-radius: 6px; padding: 22px; margin-bottom: 26px;">
			<div style="font-size: 12px; color: #64748B; margin-bottom: 4px; letter-spacing: 1px; text-transform: uppercase;">Your Admin Email</div>
			<div style="font-size: 18px; font-weight: 600; color: #0A192F;">%s</div>
		</div>

		<p style="margin: 0 0 18px 0; font-size: 15px; line-height: 24px; color: #374151;">
			Click the button below to set up your password and activate your account. You will only need to choose a password — your email and role are pre-configured.
		</p>

		<table width="100%%" cellpadding="0" cellspacing="0" border="0" style="margin-bottom: 22px;">
			<tr>
				<td align="center">
					<a href="%s" style="display: inline-block; background-color: #ff5400; color: #FFFFFF; font-size: 15px; font-weight: 600; text-decoration: none; padding: 14px 44px; border-radius: 6px; text-align: center;">
						Set Up Your Account →
					</a>
				</td>
			</tr>
		</table>

		<p style="margin: 0 0 10px 0; font-size: 14px; color: #475569;">
			This invite link expires in <strong>48 hours</strong>.
		</p>

		<div style="margin-top: 18px; background: #f1f5f9; border-radius: 6px; padding: 12px 16px;">
			<p style="margin: 0; font-size: 11px; color: #94a3b8; word-break: break-all;">%s</p>
		</div>

		<p style="margin-top: 16px; font-size: 13px; color: #94A3B8;">
			If you were not expecting this invitation, you may safely ignore this email.
		</p>
	`, role, recipientEmail, inviteLink, inviteLink)

	return BaseEmailTemplate("Admin Invitation — Flowwithlit", content)
}

// GetKYCReminderEmail nudges a user to complete identity verification.
func GetKYCReminderEmail(firstName string, kycURL string, currentTier int) string {
	tierNote := "You have not started verification yet."
	if currentTier == 1 {
		tierNote = "You are on Tier 1 — complete Tier 2 to unlock higher limits and full merchant features."
	}

	content := fmt.Sprintf(`
		<h2 style="margin: 0 0 15px 0; font-size: 20px; color: #0A192F;">Complete Your Verification</h2>
		<p style="margin: 0 0 20px 0; font-size: 15px; line-height: 24px; color: #4A4A4A;">
			Hello %s,<br><br>
			Your Flowwithlit account needs identity verification to access transfers, higher limits, and merchant tools.<br><br>
			%s
		</p>
		<table width="100%%" cellpadding="0" cellspacing="0" border="0" style="margin-bottom: 20px;">
			<tr>
				<td align="center">
					<a href="%s" class="btn-gold" style="display: block; width: 100%%; background-color: #F2A900; color: #0A192F; font-size: 15px; font-weight: 600; text-decoration: none; padding: 14px 0; border-radius: 4px; text-align: center;">
						Complete KYC Now
					</a>
				</td>
			</tr>
		</table>
		<p style="margin: 0; font-size: 14px; line-height: 22px; color: #999999;">
			Verification usually takes a few minutes. If you have already submitted documents, you can ignore this reminder.
		</p>
	`, firstName, tierNote, kycURL)

	return BaseEmailTemplate("Complete Your Verification", content)
}

// GetSecurityCheckupEmail prompts a user to review account security settings.
func GetSecurityCheckupEmail(firstName string, settingsURL string) string {
	content := fmt.Sprintf(`
		<h2 style="margin: 0 0 15px 0; font-size: 20px; color: #0A192F;">Secure Your Account</h2>
		<p style="margin: 0 0 20px 0; font-size: 15px; line-height: 24px; color: #4A4A4A;">
			Hello %s,<br><br>
			We recommend a quick security checkup on your Flowwithlit account. Please confirm the following:
		</p>
		<ul style="margin: 0 0 20px 0; padding-left: 20px; font-size: 14px; line-height: 24px; color: #4A4A4A;">
			<li>Your transaction PIN is set and known only to you</li>
			<li>Two-factor authentication (2FA) is enabled</li>
			<li>You recognise all active sessions and devices</li>
		</ul>
		<table width="100%%" cellpadding="0" cellspacing="0" border="0" style="margin-bottom: 20px;">
			<tr>
				<td align="center">
					<a href="%s" class="btn-gold" style="display: block; width: 100%%; background-color: #F2A900; color: #0A192F; font-size: 15px; font-weight: 600; text-decoration: none; padding: 14px 0; border-radius: 4px; text-align: center;">
						Review Security Settings
					</a>
				</td>
			</tr>
		</table>
		<p style="margin: 0; font-size: 14px; line-height: 22px; color: #999999;">
			If you recently updated your security settings, no further action is needed.
		</p>
	`, firstName, settingsURL)

	return BaseEmailTemplate("Secure Your Account", content)
}

// GetAccountActivationEmail encourages users to finish onboarding and fund their wallet.
func GetAccountActivationEmail(firstName string, dashboardURL string) string {
	content := fmt.Sprintf(`
		<h2 style="margin: 0 0 15px 0; font-size: 20px; color: #0A192F;">Activate Your Wallet</h2>
		<p style="margin: 0 0 20px 0; font-size: 15px; line-height: 24px; color: #4A4A4A;">
			Hello %s,<br><br>
			Your Flowwithlit account is ready — take a moment to activate it and start moving money with confidence.
		</p>
		<ul style="margin: 0 0 20px 0; padding-left: 20px; font-size: 14px; line-height: 24px; color: #4A4A4A;">
			<li>Fund your NGN or USDT wallet</li>
			<li>Set up your FlowTag for instant transfers</li>
			<li>Explore payments, cards, and developer tools</li>
		</ul>
		<table width="100%%" cellpadding="0" cellspacing="0" border="0" style="margin-bottom: 20px;">
			<tr>
				<td align="center">
					<a href="%s" class="btn-gold" style="display: block; width: 100%%; background-color: #F2A900; color: #0A192F; font-size: 15px; font-weight: 600; text-decoration: none; padding: 14px 0; border-radius: 4px; text-align: center;">
						Go to Dashboard
					</a>
				</td>
			</tr>
		</table>
		<p style="margin: 0; font-size: 14px; line-height: 22px; color: #999999;">
			Need help getting started? Reply to this email and our team will assist you.
		</p>
	`, firstName, dashboardURL)

	return BaseEmailTemplate("Activate Your Wallet", content)
}

