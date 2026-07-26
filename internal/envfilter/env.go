package envfilter

import (
	"net/http"
	"strings"

	"flowwithlit/internal/database"
	"flowwithlit/internal/models"
	"flowwithlit/internal/settlement"

	"gorm.io/gorm"
)

// Parse reads dashboard environment from query ?env=live|test or header X-Flow-Env.
// Defaults to live so production views hide sandbox data unless explicitly in test mode.
func Parse(r *http.Request) string {
	env := strings.ToLower(strings.TrimSpace(r.URL.Query().Get("env")))
	if env == "" {
		env = strings.ToLower(strings.TrimSpace(r.Header.Get("X-Flow-Env")))
	}
	if env == "test" {
		return "test"
	}
	return "live"
}

func isTestCondition() string {
	return IsTestTxnSQL("")
}

func isTestArgs() []interface{} {
	return []interface{}{true, "test"}
}

// IsTestTxnSQL matches sandbox rows on a transactions table (optional alias prefix).
func IsTestTxnSQL(table string) string {
	if table != "" {
		table += "."
	}
	return "(" + table + "is_test = ? OR " + table + "provider = ?)"
}

// IsTestTxnArgs are bind args for IsTestTxnSQL.
func IsTestTxnArgs() []interface{} {
	return isTestArgs()
}

// ApplyTxnFilter scopes transaction queries to live or test sandbox records.
func ApplyTxnFilter(db *gorm.DB, env string) *gorm.DB {
	if env == "test" {
		return db.Where(isTestCondition(), isTestArgs()...)
	}
	return db.Where("NOT "+isTestCondition(), isTestArgs()...)
}

func userSettlementDefaults(userID uint) (fiat, crypto string) {
	var user models.User
	if err := database.DB.Select("default_fiat_currency, default_crypto_currency").First(&user, userID).Error; err != nil {
		return settlement.DefaultFiat, settlement.DefaultCrypto
	}
	return settlement.UserDefaults(&user)
}

// SandboxBalances sums settled checkout amounts into the user's default fiat & crypto wallets.
func SandboxBalances(userID uint) map[string]float64 {
	fiatDef, cryptoDef := userSettlementDefaults(userID)
	balances := map[string]float64{fiatDef: 0, cryptoDef: 0}

	var user models.User
	database.DB.Select("default_fiat_currency, default_crypto_currency").First(&user, userID)

	var txns []models.Transaction
	database.DB.
		Where("user_id = ?", userID).
		Where(isTestCondition(), isTestArgs()...).
		Where("LOWER(status) = ?", "successful").
		Find(&txns)

	for _, t := range txns {
		amt, cur := settlement.ResolveSettled(&t, &user)
		if _, ok := balances[cur]; !ok {
			balances[cur] = 0
		}
		balances[cur] += amt
	}
	return balances
}

// LiveBalances returns real spendable wallet balances (all currencies).
// Never includes sandbox/test checkout totals.
func LiveBalances(userID uint) map[string]float64 {
	fiatDef, cryptoDef := userSettlementDefaults(userID)
	balances := map[string]float64{fiatDef: 0, cryptoDef: 0}

	var wallets []models.Wallet
	database.DB.Where("user_id = ?", userID).Find(&wallets)
	for _, w := range wallets {
		code := strings.ToUpper(strings.TrimSpace(w.Currency))
		if code == "" {
			continue
		}
		balances[code] = w.Balance
	}
	return balances
}

// BalancesForEnv returns spendable LIVE balances for any env.
// Sandbox totals are never mixed into spendable maps — use SandboxBalances separately.
// Deprecated for wallet display: prefer LiveBalances + optional SandboxBalances.
func BalancesForEnv(userID uint, env string) map[string]float64 {
	_ = env
	return LiveBalances(userID)
}