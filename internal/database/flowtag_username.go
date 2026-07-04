package database

import (
	"crypto/rand"
	"fmt"
	"math/big"
	"regexp"
	"strings"

	"flowwithlit/internal/models"
)

var nonAlnum = regexp.MustCompile(`[^a-z0-9]+`)

// GenerateFlowTagUsername creates a unique handle like flow_alex_4821.
func GenerateFlowTagUsername(firstName string) (string, error) {
	base := sanitizeFlowTagName(firstName)
	if base == "" {
		base = "user"
	}
	if len(base) > 12 {
		base = base[:12]
	}

	for i := 0; i < 20; i++ {
		suffix, err := randomFlowTagSuffix()
		if err != nil {
			return "", err
		}
		username := fmt.Sprintf("flow_%s_%s", base, suffix)
		var count int64
		if err := DB.Model(&models.User{}).Where("flow_tag_username = ?", username).Count(&count).Error; err != nil {
			return "", err
		}
		if count == 0 {
			return username, nil
		}
	}
	return "", fmt.Errorf("could not generate unique flowtag username")
}

func sanitizeFlowTagName(name string) string {
	s := strings.ToLower(strings.TrimSpace(name))
	return nonAlnum.ReplaceAllString(s, "")
}

func randomFlowTagSuffix() (string, error) {
	n, err := rand.Int(rand.Reader, big.NewInt(9000))
	if err != nil {
		return "", err
	}
	return fmt.Sprintf("%04d", n.Int64()+1000), nil
}

// BackfillFlowTagUsernames assigns handles to existing users missing one.
func BackfillFlowTagUsernames() {
	var users []models.User
	DB.Where("flow_tag_username = '' OR flow_tag_username IS NULL").Find(&users)
	for _, u := range users {
		username, err := GenerateFlowTagUsername(u.FirstName)
		if err != nil {
			continue
		}
		DB.Model(&u).Update("flow_tag_username", username)
	}
}