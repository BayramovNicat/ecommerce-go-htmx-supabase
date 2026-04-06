package middleware

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
	"strings"

	"htmxshop/internal/database"
)

// SupabaseClaims represents the claims in a Supabase JWT
type SupabaseClaims struct {
	Sub   string `json:"sub"`
	Email string `json:"email"`
	Role  string `json:"role"`
}

type supabaseUserResponse struct {
	ID           string                 `json:"id"`
	Email        string                 `json:"email"`
	UserMetadata map[string]interface{} `json:"user_metadata"`
}

// VerifySupabaseToken verifies a Supabase JWT token and returns the user data
func VerifySupabaseToken(tokenString string) (*supabaseUserResponse, error) {
	supabaseURL := strings.TrimSuffix(os.Getenv("SUPABASE_URL"), "/")
	if supabaseURL == "" {
		return nil, fmt.Errorf("SUPABASE_URL not configured")
	}

	serviceKey := os.Getenv("SUPABASE_SERVICE_ROLE_KEY")
	if serviceKey == "" {
		return nil, fmt.Errorf("SUPABASE_SERVICE_ROLE_KEY not configured")
	}

	req, err := http.NewRequest(http.MethodGet, supabaseURL+"/auth/v1/user", nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Authorization", "Bearer "+tokenString)
	req.Header.Set("apikey", serviceKey)
	req.Header.Set("Accept", "application/json")

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("supabase user request failed: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(io.LimitReader(resp.Body, 512))
		return nil, fmt.Errorf("supabase user request failed: status %d: %s", resp.StatusCode, strings.TrimSpace(string(body)))
	}

	var user supabaseUserResponse
	if err := json.NewDecoder(resp.Body).Decode(&user); err != nil {
		return nil, fmt.Errorf("failed to decode supabase user response: %w", err)
	}

	if user.ID == "" {
		return nil, errors.New("supabase user response missing id")
	}

	return &user, nil
}

// ExtractToken extracts the JWT token from the Authorization header or cookie
func ExtractToken(authHeader string, cookieValue string) string {
	if authHeader != "" {
		return strings.TrimPrefix(authHeader, "Bearer ")
	}
	return cookieValue
}

// VerifyAdminAccess verifies that a user has admin privileges
func VerifyAdminAccess(ctx context.Context, userID string) (bool, error) {
	return database.VerifyAdmin(ctx, userID)
}

// (legacy decode fns removed)
