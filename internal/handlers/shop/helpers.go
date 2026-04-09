package shop

import (
	"net/http"
	"os"

	"htmxshop/internal/middleware"
)

const productsPerPage = 60

func getEnv() string {
	env := os.Getenv("ENV")
	if env == "" {
		return "development"
	}
	return env
}

func getUserFromRequest(r *http.Request) map[string]interface{} {
	cookie, err := r.Cookie("sb-access-token")
	if err != nil || cookie.Value == "" {
		return nil
	}

	userData, err := middleware.VerifySupabaseToken(cookie.Value)
	if err != nil {
		return nil
	}

	return map[string]interface{}{
		"id":            userData.ID,
		"email":         userData.Email,
		"user_metadata": userData.UserMetadata,
	}
}
