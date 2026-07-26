package onepipe

import (
	"bytes"
	"crypto/md5"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"strconv"
	"strings"
	"time"
)

const defaultBaseURL = "https://api.onepipe.io/v2"

// Client handles server-side OnePipe API calls only (never expose keys or branding to browsers).
type Client struct {
	APIKey       string
	SecretKey    string
	BaseURL      string
	AuthProvider string // e.g. PolarisVirtual, FidelityVirtual — set in Admin
	MockMode     string // "live" | "inspect"
}

// NewClient initializes a OnePipe integration client.
func NewClient(apiKey, secretKey, baseURL string) *Client {
	if strings.TrimSpace(baseURL) == "" {
		baseURL = defaultBaseURL
	}
	return &Client{
		APIKey:    strings.TrimSpace(apiKey),
		SecretKey: strings.TrimSpace(secretKey),
		BaseURL:   strings.TrimRight(baseURL, "/"),
		MockMode:  "live",
	}
}

// WithAuthProvider sets the rail provider name used in auth.auth_provider.
func (c *Client) WithAuthProvider(p string) *Client {
	c.AuthProvider = strings.TrimSpace(p)
	return c
}

// WithMockMode sets transaction.mock_mode ("live" or "inspect").
func (c *Client) WithMockMode(mode string) *Client {
	mode = strings.ToLower(strings.TrimSpace(mode))
	if mode == "inspect" || mode == "live" {
		c.MockMode = mode
	}
	return c
}

// Configured reports whether live OnePipe keys are set in admin.
func (c *Client) Configured() bool {
	return c.APIKey != "" && c.SecretKey != ""
}

func (c *Client) requireKeys() error {
	if !c.Configured() {
		return fmt.Errorf("bank collection rail is not configured — contact support")
	}
	return nil
}

func (c *Client) authProviderOr(fallback string) string {
	if c.AuthProvider != "" {
		return c.AuthProvider
	}
	if fallback != "" {
		return fallback
	}
	return "PolarisVirtual"
}

// Signature is MD5(request_ref + ";" + secret) per OnePipe docs.
func Signature(requestRef, secret string) string {
	sum := md5.Sum([]byte(requestRef + ";" + secret))
	return hex.EncodeToString(sum[:])
}

// UniqueRef returns a numeric-looking unique ref (OnePipe prefers unique refs).
func UniqueRef() string {
	// 14–16 digit numeric string
	n := time.Now().UnixNano()
	return strconv.FormatInt(n%1e15, 10)
}

// Customer is identity data for open_account / lookups.
type Customer struct {
	Ref       string
	FirstName string
	LastName  string
	Email     string
	Phone     string
	Address   string
	DOB       string // preferred: yyyy-MM-dd
	Gender    string // M | F
	Title     string
}

// VirtualAccount is the result of open_account.
type VirtualAccount struct {
	AccountNumber string
	BankName      string
	BankCode      string
	AccountName   string
	Reference     string
	Provider      string
	Raw           map[string]interface{}
}

// IdentityResult is a simplified BVN/NIN lookup result.
type IdentityResult struct {
	OK        bool
	FirstName string
	LastName  string
	MiddleName string
	DOB       string
	Gender    string
	Phone     string
	Address   string
	BVN       string
	NIN       string
	Message   string
	Raw       map[string]interface{}
}

type transactRequest struct {
	RequestRef  string                 `json:"request_ref"`
	RequestType string                 `json:"request_type"`
	Auth        map[string]interface{} `json:"auth"`
	Transaction map[string]interface{} `json:"transaction"`
}

type transactResponse struct {
	Status  string                 `json:"status"`
	Message string                 `json:"message"`
	Data    map[string]interface{} `json:"data"`
}

func (c *Client) transact(requestType string, auth map[string]interface{}, txn map[string]interface{}) (*transactResponse, error) {
	if err := c.requireKeys(); err != nil {
		return nil, err
	}
	reqRef := UniqueRef()
	body := transactRequest{
		RequestRef:  reqRef,
		RequestType: requestType,
		Auth:        auth,
		Transaction: txn,
	}
	raw, err := json.Marshal(body)
	if err != nil {
		return nil, err
	}

	url := c.BaseURL + "/transact"
	httpReq, err := http.NewRequest(http.MethodPost, url, bytes.NewReader(raw))
	if err != nil {
		return nil, err
	}
	httpReq.Header.Set("Content-Type", "application/json")
	httpReq.Header.Set("Authorization", "Bearer "+c.APIKey)
	httpReq.Header.Set("Signature", Signature(reqRef, c.SecretKey))

	client := &http.Client{Timeout: 45 * time.Second}
	resp, err := client.Do(httpReq)
	if err != nil {
		return nil, fmt.Errorf("bank rail request failed: %w", err)
	}
	defer resp.Body.Close()
	respBody, _ := io.ReadAll(io.LimitReader(resp.Body, 2<<20))

	var out transactResponse
	if err := json.Unmarshal(respBody, &out); err != nil {
		return nil, fmt.Errorf("bank rail invalid response (HTTP %d)", resp.StatusCode)
	}
	// Attach HTTP status context in message when empty
	if out.Status == "" && resp.StatusCode >= 400 {
		out.Status = "Failed"
		out.Message = fmt.Sprintf("HTTP %d: %s", resp.StatusCode, strings.TrimSpace(string(respBody)))
	}
	return &out, nil
}

func normalizePhone(phone string) string {
	p := strings.TrimSpace(phone)
	p = strings.ReplaceAll(p, " ", "")
	p = strings.ReplaceAll(p, "-", "")
	p = strings.TrimPrefix(p, "+")
	if strings.HasPrefix(p, "0") && len(p) == 11 {
		p = "234" + p[1:]
	}
	return p
}

func mockMode(c *Client) string {
	if c.MockMode == "inspect" {
		return "inspect"
	}
	return "live"
}

// GenerateVirtualAccount opens a dedicated NGN virtual account (server-side only).
func (c *Client) GenerateVirtualAccount(firstName, lastName, email, phone string) (string, string, error) {
	va, err := c.OpenVirtualAccount(Customer{
		FirstName: firstName,
		LastName:  lastName,
		Email:     email,
		Phone:     phone,
	})
	if err != nil {
		return "", "", err
	}
	return va.AccountNumber, va.BankName, nil
}

// OpenVirtualAccount is the full open_account call.
func (c *Client) OpenVirtualAccount(cust Customer) (*VirtualAccount, error) {
	if err := c.requireKeys(); err != nil {
		return nil, err
	}
	ref := strings.TrimSpace(cust.Ref)
	if ref == "" {
		ref = UniqueRef()
	}
	first := strings.TrimSpace(cust.FirstName)
	last := strings.TrimSpace(cust.LastName)
	if first == "" {
		first = "Customer"
	}
	if last == "" {
		last = "User"
	}
	nameOnAccount := strings.TrimSpace(first + " " + last)
	phone := normalizePhone(cust.Phone)

	auth := map[string]interface{}{
		"type":          nil,
		"secure":        nil,
		"auth_provider": c.authProviderOr(""),
		"route_mode":    nil,
	}

	details := map[string]interface{}{
		"name_on_account": nameOnAccount,
		"country":         "NG",
	}
	if cust.Address != "" {
		details["address_line_1"] = cust.Address
	}
	if cust.DOB != "" {
		// OnePipe samples use yyyy-MM-dd-HH-mm-ss; also accept yyyy-MM-dd
		dob := cust.DOB
		if len(dob) == 10 {
			dob = dob + "-00-00-00"
		}
		details["dob"] = dob
	}
	if cust.Gender != "" {
		g := strings.ToUpper(cust.Gender)
		if strings.HasPrefix(g, "F") {
			details["gender"] = "F"
		} else {
			details["gender"] = "M"
		}
	}
	if cust.Title != "" {
		details["title"] = cust.Title
	}

	txn := map[string]interface{}{
		"mock_mode":             mockMode(c),
		"transaction_ref":       UniqueRef(),
		"transaction_desc":      "Open deposit account",
		"transaction_ref_parent": nil,
		"amount":                0,
		"customer": map[string]interface{}{
			"customer_ref": ref,
			"firstname":    first,
			"surname":      last,
			"email":        strings.TrimSpace(cust.Email),
			"mobile_no":    phone,
		},
		"meta":    map[string]interface{}{},
		"details": details,
	}

	log.Printf("[OnePipe] open_account provider=%s customer_ref=%s email=%s", c.authProviderOr(""), ref, cust.Email)
	out, err := c.transact("open_account", auth, txn)
	if err != nil {
		return nil, err
	}

	status := strings.ToLower(strings.TrimSpace(out.Status))
	if status == "waitingforotp" {
		return nil, fmt.Errorf("bank requires OTP to open account — complete host-bank setup for API-only virtual accounts")
	}
	if status != "successful" && status != "success" {
		msg := strings.TrimSpace(out.Message)
		if msg == "" {
			msg = "could not open deposit account"
		}
		return nil, fmt.Errorf("%s", msg)
	}

	pr := mapFromAny(out.Data["provider_response"])
	if pr == nil {
		// some providers nest differently
		pr = out.Data
	}
	acct := firstString(pr, "account_number", "AccountNumber", "nuban")
	bank := firstString(pr, "bank_name", "BankName", "bank")
	if acct == "" {
		// alt_accounts[0]
		if meta := mapFromAny(pr["meta"]); meta != nil {
			if alts, ok := meta["alt_accounts"].([]interface{}); ok && len(alts) > 0 {
				if a0 := mapFromAny(alts[0]); a0 != nil {
					acct = firstString(a0, "account_number")
					if bank == "" {
						bank = firstString(a0, "bank_name")
					}
				}
			}
		}
	}
	if acct == "" {
		return nil, fmt.Errorf("bank opened account but returned no account number")
	}
	if bank == "" {
		bank = "Nigerian Bank"
	}

	return &VirtualAccount{
		AccountNumber: acct,
		BankName:      bank,
		BankCode:      firstString(pr, "bank_code", "BankCode"),
		AccountName:   firstString(pr, "account_name", "AccountName", "name_on_account"),
		Reference:     firstString(pr, "reference", "account_reference"),
		Provider:      firstString(out.Data, "provider"),
		Raw:           pr,
	}, nil
}

// LookupBVN verifies a Nigerian BVN (server-side). Uses lookup_bvn_mid when possible.
func (c *Client) LookupBVN(bvn string, cust Customer) (*IdentityResult, error) {
	bvn = strings.TrimSpace(bvn)
	if len(bvn) != 11 {
		return nil, fmt.Errorf("BVN must be 11 digits")
	}
	auth := map[string]interface{}{
		"type":          "bvn",
		"secure":        bvn,
		"auth_provider": c.authProviderOr("DemoProvider"),
		"route_mode":    nil,
	}
	txn := map[string]interface{}{
		"mock_mode":       mockMode(c),
		"transaction_ref": UniqueRef(),
		"transaction_desc": "BVN verification",
		"transaction_ref_parent": nil,
		"amount":          0,
		"customer": map[string]interface{}{
			"customer_ref": nonEmpty(cust.Ref, UniqueRef()),
			"firstname":    nonEmpty(cust.FirstName, "User"),
			"surname":      nonEmpty(cust.LastName, "Customer"),
			"email":        cust.Email,
			"mobile_no":    normalizePhone(cust.Phone),
		},
		"meta":    map[string]interface{}{},
		"details": map[string]interface{}{},
	}
	if cust.DOB != "" {
		txn["details"] = map[string]interface{}{"dob": cust.DOB}
	}

	out, err := c.transact("lookup_bvn_mid", auth, txn)
	if err != nil {
		return nil, err
	}
	return parseIdentity(out, "bvn")
}

// LookupNIN verifies a Nigerian NIN (server-side).
func (c *Client) LookupNIN(nin string, cust Customer) (*IdentityResult, error) {
	nin = strings.TrimSpace(nin)
	if len(nin) < 10 {
		return nil, fmt.Errorf("invalid NIN")
	}
	auth := map[string]interface{}{
		"type":          "nin",
		"secure":        nin,
		"auth_provider": c.authProviderOr("DemoProvider"),
		"route_mode":    nil,
	}
	txn := map[string]interface{}{
		"mock_mode":       mockMode(c),
		"transaction_ref": UniqueRef(),
		"transaction_desc": "NIN verification",
		"transaction_ref_parent": nil,
		"amount":          0,
		"customer": map[string]interface{}{
			"customer_ref": nonEmpty(cust.Ref, UniqueRef()),
			"firstname":    nonEmpty(cust.FirstName, "User"),
			"surname":      nonEmpty(cust.LastName, "Customer"),
			"email":        cust.Email,
			"mobile_no":    normalizePhone(cust.Phone),
		},
		"meta":    map[string]interface{}{},
		"details": map[string]interface{}{},
	}

	out, err := c.transact("lookup_nin_mid", auth, txn)
	if err != nil {
		return nil, err
	}
	return parseIdentity(out, "nin")
}

// BankItem is a bank for transfer UI (same shape as Flutterwave list).
type BankItem struct {
	Code string `json:"code"`
	Name string `json:"name"`
}

// ListBanks tries OnePipe get_banks; returns nil slice on failure (caller may use static list).
func (c *Client) ListBanks() ([]BankItem, error) {
	if err := c.requireKeys(); err != nil {
		return nil, err
	}
	auth := map[string]interface{}{
		"type":          nil,
		"secure":        nil,
		"auth_provider": c.authProviderOr(""),
		"route_mode":    nil,
	}
	txn := map[string]interface{}{
		"mock_mode":              mockMode(c),
		"transaction_ref":        UniqueRef(),
		"transaction_desc":       "List banks",
		"transaction_ref_parent": nil,
		"amount":                 0,
		"customer": map[string]interface{}{
			"customer_ref": UniqueRef(),
			"firstname":    "Flowwithlit",
			"surname":      "System",
			"email":        "system@flowwithlit.com",
			"mobile_no":    "2348000000000",
		},
		"meta":    map[string]interface{}{},
		"details": nil,
	}
	out, err := c.transact("get_banks", auth, txn)
	if err != nil {
		return nil, err
	}
	status := strings.ToLower(strings.TrimSpace(out.Status))
	if status != "successful" && status != "success" {
		return nil, fmt.Errorf("%s", nonEmpty(out.Message, "could not list banks"))
	}
	pr := mapFromAny(out.Data["provider_response"])
	if pr == nil {
		return nil, fmt.Errorf("no banks in response")
	}
	rawBanks, ok := pr["banks"].([]interface{})
	if !ok {
		// sometimes data is a flat array under "data"
		return nil, fmt.Errorf("unexpected banks format")
	}
	banks := make([]BankItem, 0, len(rawBanks))
	for _, item := range rawBanks {
		m := mapFromAny(item)
		if m == nil {
			continue
		}
		code := firstString(m, "bank_code", "code", "cbn_code")
		name := firstString(m, "bank_name", "name")
		if code != "" && name != "" {
			banks = append(banks, BankItem{Code: code, Name: name})
		}
	}
	if len(banks) == 0 {
		return nil, fmt.Errorf("empty bank list")
	}
	return banks, nil
}

// ResolveBankAccount performs NUBAN name enquiry (account number + bank code).
func (c *Client) ResolveBankAccount(bankCode, accountNumber string) (string, error) {
	if err := c.requireKeys(); err != nil {
		return "", err
	}
	accountNumber = strings.TrimSpace(accountNumber)
	bankCode = strings.TrimSpace(bankCode)
	if len(accountNumber) != 10 {
		return "", fmt.Errorf("account number must be 10 digits")
	}
	if bankCode == "" {
		return "", fmt.Errorf("bank is required")
	}

	auth := map[string]interface{}{
		"type":          nil,
		"secure":        nil,
		"auth_provider": c.authProviderOr(""),
		"route_mode":    nil,
	}
	txn := map[string]interface{}{
		"mock_mode":              mockMode(c),
		"transaction_ref":        UniqueRef(),
		"transaction_desc":       "Name enquiry",
		"transaction_ref_parent": nil,
		"amount":                 0,
		"customer": map[string]interface{}{
			"customer_ref": UniqueRef(),
			"firstname":    "Flowwithlit",
			"surname":      "User",
			"email":        "system@flowwithlit.com",
			"mobile_no":    "2348000000000",
		},
		"meta": map[string]interface{}{},
		"details": map[string]interface{}{
			"account_number": accountNumber,
			"bank_code":      bankCode,
		},
	}
	out, err := c.transact("lookup_nuban", auth, txn)
	if err != nil {
		return "", err
	}
	status := strings.ToLower(strings.TrimSpace(out.Status))
	if status != "successful" && status != "success" {
		return "", fmt.Errorf("%s", nonEmpty(out.Message, "account could not be verified"))
	}
	pr := mapFromAny(out.Data["provider_response"])
	if pr == nil {
		return "", fmt.Errorf("account could not be verified")
	}
	// Prefer direct account_name fields
	name := firstString(pr, "account_name", "AccountName", "name", "beneficiary_account_name")
	if name != "" {
		return name, nil
	}
	// Some rails return banks[] with name when account is known
	if banks, ok := pr["banks"].([]interface{}); ok {
		for _, b := range banks {
			m := mapFromAny(b)
			if m == nil {
				continue
			}
			code := firstString(m, "bank_code", "code")
			if code == bankCode || code == "" {
				if n := firstString(m, "account_name", "bank_name"); n != "" && !strings.EqualFold(n, firstString(m, "bank_name")) {
					// only use if looks like person name field
				}
				if n := firstString(m, "account_name"); n != "" {
					return n, nil
				}
			}
		}
	}
	return "", fmt.Errorf("account name not returned — check number and bank")
}

// ProcessTransfer sends NGN to an external bank account via disburse.
// amountNaira is in major units (naira); converted to kobo for the API.
func (c *Client) ProcessTransfer(amount float64, bankCode, accountNumber, narration string) (bool, string, error) {
	if err := c.requireKeys(); err != nil {
		return false, "", err
	}
	if amount <= 0 {
		return false, "", fmt.Errorf("invalid amount")
	}
	kobo := int64(amount*100 + 0.5)
	auth := map[string]interface{}{
		"type":          nil,
		"secure":        nil,
		"auth_provider": c.authProviderOr(""),
		"route_mode":    nil,
	}
	txnRef := UniqueRef()
	txn := map[string]interface{}{
		"mock_mode":       mockMode(c),
		"transaction_ref": txnRef,
		"transaction_desc": nonEmpty(narration, "Transfer"),
		"transaction_ref_parent": nil,
		"amount":          kobo,
		"customer": map[string]interface{}{
			"customer_ref": UniqueRef(),
			"firstname":    "Flowwithlit",
			"surname":      "Payout",
			"email":        "payouts@flowwithlit.com",
			"mobile_no":    "2348000000000",
		},
		"meta": map[string]interface{}{},
		"details": map[string]interface{}{
			"destination_account":  strings.TrimSpace(accountNumber),
			"destination_bank_code": strings.TrimSpace(bankCode),
		},
	}

	out, err := c.transact("disburse", auth, txn)
	if err != nil {
		return false, "", err
	}
	status := strings.ToLower(strings.TrimSpace(out.Status))
	if status != "successful" && status != "success" {
		msg := strings.TrimSpace(out.Message)
		if msg == "" {
			msg = "transfer failed"
		}
		return false, "", fmt.Errorf("%s", msg)
	}
	pr := mapFromAny(out.Data["provider_response"])
	ref := txnRef
	if pr != nil {
		if r := firstString(pr, "reference", "payment_id"); r != "" {
			ref = r
		}
	}
	return true, ref, nil
}

func parseIdentity(out *transactResponse, kind string) (*IdentityResult, error) {
	status := strings.ToLower(strings.TrimSpace(out.Status))
	if status == "waitingforotp" {
		return &IdentityResult{OK: false, Message: "OTP required for identity verification"}, fmt.Errorf("identity verification requires OTP — use host-bank product that supports silent API lookup, or complete OTP flow")
	}
	if status != "successful" && status != "success" {
		msg := strings.TrimSpace(out.Message)
		if msg == "" {
			msg = "identity verification failed"
		}
		return &IdentityResult{OK: false, Message: msg}, fmt.Errorf("%s", msg)
	}
	pr := mapFromAny(out.Data["provider_response"])
	if pr == nil {
		return &IdentityResult{OK: true, Message: out.Message}, nil
	}
	res := &IdentityResult{
		OK:         true,
		FirstName:  firstString(pr, "first_name", "firstname"),
		LastName:   firstString(pr, "last_name", "surname", "lastname"),
		MiddleName: firstString(pr, "middle_name", "middlename"),
		DOB:        firstString(pr, "dob", "date_of_birth"),
		Gender:     firstString(pr, "gender"),
		Phone:      firstString(pr, "phone_number1", "phone_number", "mobile_no"),
		Address:    firstString(pr, "residential_address", "address"),
		BVN:        firstString(pr, "bvn"),
		NIN:        firstString(pr, "nin"),
		Message:    out.Message,
		Raw:        pr,
	}
	// min lookups return booleans for name match — still treat success status as OK
	if kind == "bvn" && res.BVN == "" {
		res.BVN = firstString(pr, "bvn")
	}
	return res, nil
}

func mapFromAny(v interface{}) map[string]interface{} {
	if v == nil {
		return nil
	}
	if m, ok := v.(map[string]interface{}); ok {
		return m
	}
	return nil
}

func firstString(m map[string]interface{}, keys ...string) string {
	if m == nil {
		return ""
	}
	for _, k := range keys {
		if v, ok := m[k]; ok && v != nil {
			switch t := v.(type) {
			case string:
				if strings.TrimSpace(t) != "" {
					return strings.TrimSpace(t)
				}
			case float64:
				return strconv.FormatInt(int64(t), 10)
			case json.Number:
				return t.String()
			case bool:
				// min match flags — ignore
			default:
				s := strings.TrimSpace(fmt.Sprint(t))
				if s != "" && s != "<nil>" {
					return s
				}
			}
		}
	}
	return ""
}

func nonEmpty(v, fallback string) string {
	if strings.TrimSpace(v) != "" {
		return strings.TrimSpace(v)
	}
	return fallback
}
