package handlers

import "fmt"

// ─────────────────────────────────────────────────────────────────────────────
// REOS Brand Email Templates
// All templates use inline CSS for maximum email client compatibility.
// Brand palette:
//   Background : #faf8f5  (parchment)
//   Primary    : #1c1712  (charcoal)
//   Accent     : #c9973f  (champagne gold)
//   Card BG    : #ffffff
//   Muted text : #6b7280
// ─────────────────────────────────────────────────────────────────────────────

const emailBase = `
<!DOCTYPE html>
<html lang="en">
<head>
  <meta charset="UTF-8" />
  <meta name="viewport" content="width=device-width, initial-scale=1.0"/>
  <title>%s</title>
</head>
<body style="margin:0;padding:0;background:#faf8f5;font-family:'Helvetica Neue',Helvetica,Arial,sans-serif;">
  <table width="100%%" cellpadding="0" cellspacing="0" role="presentation" style="background:#faf8f5;padding:40px 16px;">
    <tr>
      <td align="center">
        <table width="100%%" cellpadding="0" cellspacing="0" role="presentation" style="max-width:560px;width:100%%;">

          <!-- HEADER / LOGO -->
          <tr>
            <td style="padding-bottom:24px;text-align:center;">
              <table cellpadding="0" cellspacing="0" role="presentation" style="display:inline-table;background:#1c1712;border-radius:14px;padding:12px 22px;">
                <tr>
                  <td style="vertical-align:middle;padding-right:10px;">
                    <div style="width:28px;height:28px;background:linear-gradient(135deg,#c9973f 0%%,#e8b96a 100%%);border-radius:7px;display:inline-block;"></div>
                  </td>
                  <td style="vertical-align:middle;">
                    <span style="font-size:17px;font-weight:700;color:#ffffff;letter-spacing:0.5px;">REOS</span>
                    <span style="font-size:10px;color:#c9973f;display:block;letter-spacing:2px;text-transform:uppercase;margin-top:-2px;">Rental OS</span>
                  </td>
                </tr>
              </table>
            </td>
          </tr>

          <!-- CARD -->
          <tr>
            <td style="background:#ffffff;border-radius:20px;padding:40px 40px 32px;box-shadow:0 2px 24px rgba(28,23,18,0.08);border:1px solid rgba(201,151,63,0.12);">
              %s
            </td>
          </tr>

          <!-- FOOTER -->
          <tr>
            <td style="padding-top:28px;text-align:center;">
              <p style="margin:0;font-size:11px;color:#9ca3af;line-height:1.7;">
                This email was sent by <strong style="color:#6b7280;">REOS — Rental Operating System</strong>.<br/>
                If you did not request this, you can safely ignore this email.
              </p>
              <p style="margin:12px 0 0;font-size:10px;color:#d1d5db;">
                &copy; 2026 REOS. All rights reserved.
              </p>
            </td>
          </tr>

        </table>
      </td>
    </tr>
  </table>
</body>
</html>
`

// goldDivider returns a thin gold-tinted horizontal rule for use inside card sections.
const goldDivider = `<div style="height:1px;background:linear-gradient(90deg,transparent,rgba(201,151,63,0.3),transparent);margin:24px 0;"></div>`

// goldBadge renders a small uppercase pill label with gold accent.
func goldBadge(text string) string {
	return fmt.Sprintf(
		`<span style="display:inline-block;background:rgba(201,151,63,0.12);color:#a07828;border:1px solid rgba(201,151,63,0.3);border-radius:50px;padding:4px 12px;font-size:10px;font-weight:700;letter-spacing:1.5px;text-transform:uppercase;">%s</span>`,
		text,
	)
}

// ctaButton renders a primary CTA button.
func ctaButton(href, label string) string {
	return fmt.Sprintf(`
<table cellpadding="0" cellspacing="0" role="presentation" style="margin:0 auto;">
  <tr>
    <td style="background:linear-gradient(135deg,#c9973f 0%%,#e8b96a 100%%);border-radius:12px;padding:1px;">
      <a href="%s" target="_blank"
         style="display:inline-block;background:#1c1712;color:#c9973f;font-size:13px;font-weight:700;
                letter-spacing:0.3px;text-decoration:none;border-radius:11px;padding:13px 32px;">
        %s &rarr;
      </a>
    </td>
  </tr>
</table>`, href, label)
}

// otpBlock renders a large styled OTP code block.
func otpBlock(code string) string {
	return fmt.Sprintf(`
<div style="background:#faf8f5;border:1px solid rgba(201,151,63,0.25);border-radius:14px;padding:20px;text-align:center;margin:24px 0;">
  <p style="margin:0 0 4px;font-size:10px;color:#9ca3af;letter-spacing:2px;text-transform:uppercase;font-weight:600;">Verification Code</p>
  <p style="margin:0;font-size:38px;font-weight:800;color:#1c1712;letter-spacing:10px;">%s</p>
</div>`, code)
}

// ─────────────────────────────────────────────────────────────────────────────
// EmailInvitation builds an invitation email with the acceptance link embedded.
// Parameters:
//   recipientEmail — the invited person's email
//   senderName     — the name/email of who sent the invite
//   propertyName   — the property they're being invited to (or "System Platform")
//   role           — tenant / caretaker / agent / etc.
//   inviteLink     — full acceptance URL with token
// ─────────────────────────────────────────────────────────────────────────────
func EmailInvitation(recipientEmail, senderName, propertyName, role, inviteLink string) string {
	inner := fmt.Sprintf(`
    <!-- Badge -->
    <div style="text-align:center;margin-bottom:28px;">%s</div>

    <!-- Heading -->
    <h1 style="margin:0 0 8px;font-size:22px;font-weight:800;color:#1c1712;text-align:center;line-height:1.3;">
      You're Invited to Join REOS
    </h1>
    <p style="margin:0;font-size:13px;color:#6b7280;text-align:center;line-height:1.7;">
      <strong style="color:#1c1712;">%s</strong> has invited you to join<br/>
      <strong style="color:#1c1712;">%s</strong> as a <strong style="color:#a07828;">%s</strong>.
    </p>

    %s

    <!-- Details block -->
    <table width="100%%" cellpadding="0" cellspacing="0" style="background:#faf8f5;border-radius:12px;border:1px solid rgba(201,151,63,0.18);margin-bottom:28px;">
      <tr>
        <td style="padding:16px 20px;">
          <table width="100%%" cellpadding="0" cellspacing="0">
            <tr>
              <td style="padding:6px 0;border-bottom:1px solid rgba(201,151,63,0.1);">
                <span style="font-size:10px;color:#9ca3af;font-weight:600;text-transform:uppercase;letter-spacing:1px;">Invited Email</span><br/>
                <span style="font-size:13px;color:#1c1712;font-weight:600;">%s</span>
              </td>
            </tr>
            <tr>
              <td style="padding:6px 0;border-bottom:1px solid rgba(201,151,63,0.1);">
                <span style="font-size:10px;color:#9ca3af;font-weight:600;text-transform:uppercase;letter-spacing:1px;">Property / Platform</span><br/>
                <span style="font-size:13px;color:#1c1712;font-weight:600;">%s</span>
              </td>
            </tr>
            <tr>
              <td style="padding:6px 0;">
                <span style="font-size:10px;color:#9ca3af;font-weight:600;text-transform:uppercase;letter-spacing:1px;">Your Role</span><br/>
                <span style="font-size:13px;color:#a07828;font-weight:700;text-transform:capitalize;">%s</span>
              </td>
            </tr>
          </table>
        </td>
      </tr>
    </table>

    <!-- CTA -->
    <div style="text-align:center;margin-bottom:24px;">
      %s
    </div>

    <!-- Link fallback -->
    <p style="margin:0;font-size:11px;color:#9ca3af;text-align:center;line-height:1.6;">
      Or paste this link in your browser:<br/>
      <a href="%s" style="color:#a07828;word-break:break-all;">%s</a>
    </p>

    %s

    <!-- Expiry warning -->
    <p style="margin:0;font-size:11px;color:#9ca3af;text-align:center;">
      ⏳ This invitation link expires in <strong style="color:#6b7280;">7 days</strong>.
    </p>
  `,
		goldBadge("Team Invitation"),
		senderName,
		propertyName,
		role,
		goldDivider,
		recipientEmail,
		propertyName,
		role,
		ctaButton(inviteLink, "Accept Invitation"),
		inviteLink,
		inviteLink,
		goldDivider,
	)
	return fmt.Sprintf(emailBase, "You're invited to join REOS", inner)
}

// ─────────────────────────────────────────────────────────────────────────────
// EmailOTPVerification builds an account verification OTP email.
// ─────────────────────────────────────────────────────────────────────────────
func EmailOTPVerification(recipientEmail, otp string) string {
	inner := fmt.Sprintf(`
    <!-- Badge -->
    <div style="text-align:center;margin-bottom:28px;">%s</div>

    <!-- Heading -->
    <h1 style="margin:0 0 8px;font-size:22px;font-weight:800;color:#1c1712;text-align:center;">
      Verify Your Account
    </h1>
    <p style="margin:0;font-size:13px;color:#6b7280;text-align:center;line-height:1.7;">
      Welcome to REOS! Enter the code below to verify<br/>
      <strong style="color:#1c1712;">%s</strong> and activate your account.
    </p>

    %s

    %s

    %s

    <p style="margin:16px 0 0;font-size:11px;color:#9ca3af;text-align:center;">
      🔒 This code expires in <strong style="color:#6b7280;">10 minutes</strong>. Do not share it with anyone.
    </p>
  `,
		goldBadge("Email Verification"),
		recipientEmail,
		goldDivider,
		otpBlock(otp),
		goldDivider,
	)
	return fmt.Sprintf(emailBase, "Verify your REOS account", inner)
}

// ─────────────────────────────────────────────────────────────────────────────
// EmailWelcome builds a post-acceptance welcome email sent after invitation accepted.
// ─────────────────────────────────────────────────────────────────────────────
func EmailWelcome(recipientName, role, dashboardLink string) string {
	inner := fmt.Sprintf(`
    <!-- Badge -->
    <div style="text-align:center;margin-bottom:28px;">%s</div>

    <!-- Heading -->
    <h1 style="margin:0 0 8px;font-size:22px;font-weight:800;color:#1c1712;text-align:center;">
      Welcome to REOS, %s!
    </h1>
    <p style="margin:0;font-size:13px;color:#6b7280;text-align:center;line-height:1.7;">
      Your account has been created and you're all set.<br/>
      You have been granted the <strong style="color:#a07828;text-transform:capitalize;">%s</strong> role.
    </p>

    %s

    <!-- Feature highlights -->
    <table width="100%%" cellpadding="0" cellspacing="0" style="margin-bottom:28px;">
      <tr>
        <td style="padding:10px;background:#faf8f5;border-radius:10px;border:1px solid rgba(201,151,63,0.15);text-align:center;">
          <p style="margin:0;font-size:22px;">🏠</p>
          <p style="margin:4px 0 0;font-size:11px;font-weight:700;color:#1c1712;">Property Management</p>
          <p style="margin:2px 0 0;font-size:10px;color:#9ca3af;">Full-stack rental operations</p>
        </td>
        <td style="width:10px;"></td>
        <td style="padding:10px;background:#faf8f5;border-radius:10px;border:1px solid rgba(201,151,63,0.15);text-align:center;">
          <p style="margin:0;font-size:22px;">📊</p>
          <p style="margin:4px 0 0;font-size:11px;font-weight:700;color:#1c1712;">Smart Analytics</p>
          <p style="margin:2px 0 0;font-size:10px;color:#9ca3af;">Real-time insights & reports</p>
        </td>
        <td style="width:10px;"></td>
        <td style="padding:10px;background:#faf8f5;border-radius:10px;border:1px solid rgba(201,151,63,0.15);text-align:center;">
          <p style="margin:0;font-size:22px;">💬</p>
          <p style="margin:4px 0 0;font-size:11px;font-weight:700;color:#1c1712;">Communications</p>
          <p style="margin:2px 0 0;font-size:10px;color:#9ca3af;">Tenants, agents & staff</p>
        </td>
      </tr>
    </table>

    <!-- CTA -->
    <div style="text-align:center;margin-bottom:24px;">
      %s
    </div>

    %s

    <p style="margin:0;font-size:11px;color:#9ca3af;text-align:center;">
      Questions? Reply to this email or reach our support team.
    </p>
  `,
		goldBadge("Welcome Aboard"),
		recipientName,
		role,
		goldDivider,
		ctaButton(dashboardLink, "Go to Dashboard"),
		goldDivider,
	)
	return fmt.Sprintf(emailBase, "Welcome to REOS", inner)
}

// ─────────────────────────────────────────────────────────────────────────────
// EmailPasswordReset builds a password reset / OTP email.
// ─────────────────────────────────────────────────────────────────────────────
func EmailPasswordReset(recipientEmail, otp string) string {
	inner := fmt.Sprintf(`
    <!-- Badge -->
    <div style="text-align:center;margin-bottom:28px;">%s</div>

    <!-- Heading -->
    <h1 style="margin:0 0 8px;font-size:22px;font-weight:800;color:#1c1712;text-align:center;">
      Reset Your Password
    </h1>
    <p style="margin:0;font-size:13px;color:#6b7280;text-align:center;line-height:1.7;">
      We received a password reset request for<br/>
      <strong style="color:#1c1712;">%s</strong>.<br/>
      Use the code below to complete your reset.
    </p>

    %s

    %s

    %s

    <p style="margin:16px 0 0;font-size:11px;color:#9ca3af;text-align:center;">
      ⚠️ If you did not request a password reset, please <strong style="color:#6b7280;">ignore this email</strong> and your account will remain secure.
    </p>
  `,
		goldBadge("Security Alert"),
		recipientEmail,
		goldDivider,
		otpBlock(otp),
		goldDivider,
	)
	return fmt.Sprintf(emailBase, "REOS Password Reset", inner)
}
