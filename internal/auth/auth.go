package auth

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
	"strings"

	"htmxshop/internal/db"
)

// SupabaseClaims represents the claims in a Supabase JWT
type SupabaseClaims struct {
	Sub   string `json:"sub"`
	Email string `json:"email"`
	Role  string `json:"role"`
}

type supabaseUserResponse struct {
	ID string `json:"id"`
}

// VerifySupabaseToken verifies a Supabase JWT token and returns the user ID
func VerifySupabaseToken(tokenString string) (string, error) {
	supabaseURL := strings.TrimSuffix(os.Getenv("SUPABASE_URL"), "/")
	if supabaseURL == "" {
		return "", fmt.Errorf("SUPABASE_URL not configured")
	}

	serviceKey := os.Getenv("SUPABASE_SERVICE_ROLE_KEY")
	if serviceKey == "" {
		return "", fmt.Errorf("SUPABASE_SERVICE_ROLE_KEY not configured")
	}

	req, err := http.NewRequest(http.MethodGet, supabaseURL+"/auth/v1/user", nil)
	if err != nil {
		return "", err
	}
	req.Header.Set("Authorization", "Bearer "+tokenString)
	req.Header.Set("apikey", serviceKey)
	req.Header.Set("Accept", "application/json")

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return "", fmt.Errorf("supabase user request failed: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(io.LimitReader(resp.Body, 512))
		return "", fmt.Errorf("supabase user request failed: status %d: %s", resp.StatusCode, strings.TrimSpace(string(body)))
	}

	var user supabaseUserResponse
	if err := json.NewDecoder(resp.Body).Decode(&user); err != nil {
		return "", fmt.Errorf("failed to decode supabase user response: %w", err)
	}

	if user.ID == "" {
		return "", errors.New("supabase user response missing id")
	}

	return user.ID, nil
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
	return db.VerifyAdmin(ctx, userID)
}

// (legacy decode fns removed)
