package bills

// Catalog of everyday bank-style bill services for Nigeria.
// Live fulfillment: VTU (SME/gifting) first for airtime/data, else Flutterwave.
// No mock success — missing keys return clear errors (see key-get.md).

type Category struct {
	ID          string `json:"id"`
	Name        string `json:"name"`
	Description string `json:"description"`
	Icon        string `json:"icon"` // hint for mobile UI
}

type Product struct {
	ID          string  `json:"id"`
	CategoryID  string  `json:"category_id"`
	Name        string  `json:"name"`
	Provider    string  `json:"provider"` // MTN, GLO, DSTV, etc.
	Amount      float64 `json:"amount"`   // 0 = user enters amount (airtime)
	Currency    string  `json:"currency"`
	BillerCode  string  `json:"biller_code,omitempty"` // Flutterwave biller when live
	ItemCode    string  `json:"item_code,omitempty"`
}

func Categories() []Category {
	return []Category{
		{ID: "airtime", Name: "Airtime", Description: "Top up any Nigerian mobile number", Icon: "phone"},
		{ID: "data", Name: "Data (SME & Gifting)", Description: "Cheaper SME/corporate and gifting data — better than bank retail rates", Icon: "wifi"},
		{ID: "electricity", Name: "Electricity", Description: "Pay prepaid/postpaid power bills", Icon: "bolt"},
		{ID: "cable", Name: "Cable TV", Description: "DSTV, GOTV, Startimes and more", Icon: "tv"},
	}
}

// Static starter products for UI. Live mode can enrich from Flutterwave billers later.
func Products(categoryID string) []Product {
	all := []Product{
		// Airtime — amount is custom
		{ID: "airtime_mtn", CategoryID: "airtime", Name: "MTN Airtime", Provider: "MTN", Amount: 0, Currency: "NGN", BillerCode: "BIL099"},
		{ID: "airtime_glo", CategoryID: "airtime", Name: "Glo Airtime", Provider: "GLO", Amount: 0, Currency: "NGN", BillerCode: "BIL102"},
		{ID: "airtime_airtel", CategoryID: "airtime", Name: "Airtel Airtime", Provider: "AIRTEL", Amount: 0, Currency: "NGN", BillerCode: "BIL100"},
		{ID: "airtime_9mobile", CategoryID: "airtime", Name: "9mobile Airtime", Provider: "9MOBILE", Amount: 0, Currency: "NGN", BillerCode: "BIL103"},

		// Data — CORPORATE/SME & GIFTING tiers (cheaper than bank/OTC retail).
		// Live: prefer SME/VTU aggregators (VTPass, ClubKonnect, SMEPlug-style) when integrated;
		// Flutterwave retail bills are fallback (usually more expensive).
		// SME = corporate data (often cheaper per GB). Gifting = shareable data gifts.
		{ID: "sme_mtn_1gb", CategoryID: "data", Name: "MTN SME 1GB (cheap)", Provider: "MTN", Amount: 280, Currency: "NGN"},
		{ID: "sme_mtn_2gb", CategoryID: "data", Name: "MTN SME 2GB (cheap)", Provider: "MTN", Amount: 560, Currency: "NGN"},
		{ID: "sme_mtn_5gb", CategoryID: "data", Name: "MTN SME 5GB (cheap)", Provider: "MTN", Amount: 1400, Currency: "NGN"},
		{ID: "gift_mtn_1gb", CategoryID: "data", Name: "MTN Gifting 1GB", Provider: "MTN", Amount: 350, Currency: "NGN"},
		{ID: "gift_mtn_2gb", CategoryID: "data", Name: "MTN Gifting 2GB", Provider: "MTN", Amount: 700, Currency: "NGN"},
		{ID: "sme_glo_1gb", CategoryID: "data", Name: "Glo SME 1.5GB (cheap)", Provider: "GLO", Amount: 300, Currency: "NGN"},
		{ID: "gift_glo_1gb", CategoryID: "data", Name: "Glo Gifting 1GB", Provider: "GLO", Amount: 400, Currency: "NGN"},
		{ID: "sme_airtel_1gb", CategoryID: "data", Name: "Airtel SME 1GB (cheap)", Provider: "AIRTEL", Amount: 300, Currency: "NGN"},
		{ID: "gift_airtel_1gb", CategoryID: "data", Name: "Airtel Gifting 1GB", Provider: "AIRTEL", Amount: 400, Currency: "NGN"},
		{ID: "sme_9mobile_1gb", CategoryID: "data", Name: "9mobile SME 1GB (cheap)", Provider: "9MOBILE", Amount: 300, Currency: "NGN"},
		// Retail fallback (higher price — bank-style)
		{ID: "data_mtn_1gb_retail", CategoryID: "data", Name: "MTN 1GB Retail", Provider: "MTN", Amount: 500, Currency: "NGN"},
		{ID: "data_mtn_2gb_retail", CategoryID: "data", Name: "MTN 2GB Retail", Provider: "MTN", Amount: 1000, Currency: "NGN"},

		// Electricity
		{ID: "elec_ikeja", CategoryID: "electricity", Name: "Ikeja Electric (IKEDC)", Provider: "IKEDC", Amount: 0, Currency: "NGN"},
		{ID: "elec_eko", CategoryID: "electricity", Name: "Eko Electric (EKEDC)", Provider: "EKEDC", Amount: 0, Currency: "NGN"},
		{ID: "elec_abuja", CategoryID: "electricity", Name: "Abuja Electric (AEDC)", Provider: "AEDC", Amount: 0, Currency: "NGN"},

		// Cable
		{ID: "cable_dstv", CategoryID: "cable", Name: "DSTV", Provider: "DSTV", Amount: 0, Currency: "NGN"},
		{ID: "cable_gotv", CategoryID: "cable", Name: "GOTV", Provider: "GOTV", Amount: 0, Currency: "NGN"},
		{ID: "cable_startimes", CategoryID: "cable", Name: "Startimes", Provider: "STARTIMES", Amount: 0, Currency: "NGN"},
	}

	if categoryID == "" {
		return all
	}
	out := make([]Product, 0)
	for _, p := range all {
		if p.CategoryID == categoryID {
			out = append(out, p)
		}
	}
	return out
}

func FindProduct(id string) *Product {
	for _, p := range Products("") {
		if p.ID == id {
			cp := p
			return &cp
		}
	}
	return nil
}
