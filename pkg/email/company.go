package email

import "flowwithlit/internal/company"

// companyTemplateVars loads company placeholders for email rendering.
func companyTemplateVars() map[string]interface{} {
	vars := make(map[string]interface{})
	for k, v := range company.Get().TemplateVars() {
		vars[k] = v
	}
	return vars
}