package admin

import (
	"encoding/json"
	"net/http"
	"strings"

	"flowwithlit/pkg/email"
	"flowwithlit/pkg/response"
)

// ListTestEmailTemplatesHandler returns all email templates with sample vars.
func ListTestEmailTemplatesHandler(w http.ResponseWriter, r *http.Request) {
	response.Success(w, http.StatusOK, map[string]interface{}{
		"templates":          email.TestEmailCatalog(),
		"flow":               []string{"PHP test page", "Go API", "PHP mail/dispatch.php", "PHPMailer"},
		"php_dispatch_url":   email.MailDispatchURL(),
		"gomail_fallback":    false,
	})
}

type sendTestEmailRequest struct {
	To       string                 `json:"to"`
	Template string                 `json:"template"`
	Subject  string                 `json:"subject"`
	Vars     map[string]interface{} `json:"vars"`
}

// SendTestEmailHandler sends a template email through Go → PHP dispatch.
func SendTestEmailHandler(w http.ResponseWriter, r *http.Request) {
	var req sendTestEmailRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		response.Error(w, http.StatusBadRequest, "Invalid request payload")
		return
	}

	to := strings.TrimSpace(req.To)
	template := strings.TrimSpace(req.Template)
	if to == "" || template == "" {
		response.Error(w, http.StatusBadRequest, "Recipient email and template are required")
		return
	}
	if !strings.Contains(to, "@") {
		response.Error(w, http.StatusBadRequest, "Invalid recipient email")
		return
	}

	subject, sampleVars, ok := email.SampleForTemplate(template)
	if !ok {
		response.Error(w, http.StatusBadRequest, "Unknown email template")
		return
	}

	if strings.TrimSpace(req.Subject) != "" {
		subject = strings.TrimSpace(req.Subject)
	}
	subject = email.TestEmailSubject(subject)

	vars := sampleVars
	if len(req.Vars) > 0 {
		vars = make(map[string]interface{}, len(sampleVars)+len(req.Vars))
		for k, v := range sampleVars {
			vars[k] = v
		}
		for k, v := range req.Vars {
			vars[k] = v
		}
	}

	if err := email.SendTemplateMail(to, subject, template, vars); err != nil {
		response.Error(w, http.StatusInternalServerError, "Failed to send test email: "+err.Error())
		return
	}

	response.Success(w, http.StatusOK, map[string]interface{}{
		"sent":              true,
		"to":                to,
		"template":          template,
		"subject":           subject,
		"via":               "php",
		"php_dispatch_url":  email.MailDispatchURL(),
		"message":           "Test email dispatched via Go → PHP mail/dispatch.php → PHPMailer",
		"flow":              []string{"PHP test page", "Go API", "PHP mail/dispatch.php", "PHPMailer"},
	})
}