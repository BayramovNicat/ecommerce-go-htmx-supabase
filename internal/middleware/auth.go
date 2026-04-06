package middleware

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"math"
	"net/http"
	"os"
	"strings"
	"sync"
	"time"

	"github.com/golang-jwt/jwt/v5"
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

type cachedAuthUser struct {
	user      *supabaseUserResponse
	expiresAt time.Time
}

var authTokenCache sync.Map

const maxAuthTokenCacheTTL = 5 * time.Minute

// VerifySupabaseToken verifies a Supabase JWT token and returns the user data
func VerifySupabaseToken(tokenString string) (*supabaseUserResponse, error) {
	if tokenString == "" {
		return nil, errors.New("missing token")
	}

	if user := getCachedAuthUser(tokenString); user != nil {
		return user, nil
	}

	jwtSecret := os.Getenv("SUPABASE_JWT_SECRET")
	if jwtSecret != "" {
		user, exp, err := verifySupabaseTokenLocally(tokenString, jwtSecret)
		if err != nil {
			return nil, err
		}
		cacheAuthUser(tokenString, user, exp)
		return user, nil
	}

	user, err := verifySupabaseTokenWithAPI(tokenString)
	if err != nil {
		return nil, err
	}

	cacheAuthUser(tokenString, user, time.Now().Add(30*time.Second))
	return user, nil
}

func verifySupabaseTokenWithAPI(tokenString string) (*supabaseUserResponse, error) {
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

func verifySupabaseTokenLocally(tokenString, jwtSecret string) (*supabaseUserResponse, time.Time, error) {
	claims := jwt.MapClaims{}
	token, err := jwt.ParseWithClaims(tokenString, claims, func(token *jwt.Token) (interface{}, error) {
		return []byte(jwtSecret), nil
	}, jwt.WithValidMethods([]string{jwt.SigningMethodHS256.Alg()}))
	if err != nil {
		return nil, time.Time{}, fmt.Errorf("token verification failed: %w", err)
	}

	if !token.Valid {
		return nil, time.Time{}, errors.New("invalid token")
	}

	sub, _ := claims["sub"].(string)
	if sub == "" {
		return nil, time.Time{}, errors.New("token missing sub claim")
	}

	user := &supabaseUserResponse{
		ID:           sub,
		Email:        claimString(claims, "email"),
		UserMetadata: claimMap(claims, "user_metadata"),
	}

	return user, claimTime(claims, "exp"), nil
}

func claimString(claims jwt.MapClaims, key string) string {
	v, _ := claims[key].(string)
	return v
}

func claimMap(claims jwt.MapClaims, key string) map[string]interface{} {
	v, ok := claims[key].(map[string]interface{})
	if !ok {
		return map[string]interface{}{}
	}
	return v
}

func claimTime(claims jwt.MapClaims, key string) time.Time {
	v, ok := claims[key]
	if !ok {
		return time.Time{}
	}

	switch exp := v.(type) {
	case float64:
		if exp <= 0 || math.IsNaN(exp) {
			return time.Time{}
		}
		return time.Unix(int64(exp), 0)
	case int64:
		if exp <= 0 {
			return time.Time{}
		}
		return time.Unix(exp, 0)
	case json.Number:
		n, err := exp.Int64()
		if err != nil || n <= 0 {
			return time.Time{}
		}
		return time.Unix(n, 0)
	default:
		return time.Time{}
	}
}

func getCachedAuthUser(tokenString string) *supabaseUserResponse {
	value, ok := authTokenCache.Load(tokenString)
	if !ok {
		return nil
	}

	entry := value.(cachedAuthUser)
	if time.Now().After(entry.expiresAt) {
		authTokenCache.Delete(tokenString)
		return nil
	}

	user := *entry.user
	return &user
}

func cacheAuthUser(tokenString string, user *supabaseUserResponse, expiresAt time.Time) {
	if user == nil {
		return
	}

	now := time.Now()
	if expiresAt.IsZero() || expiresAt.Before(now) {
		expiresAt = now.Add(30 * time.Second)
	}

	maxExpiry := now.Add(maxAuthTokenCacheTTL)
	if expiresAt.After(maxExpiry) {
		expiresAt = maxExpiry
	}

	authTokenCache.Store(tokenString, cachedAuthUser{
		user:      user,
		expiresAt: expiresAt,
	})
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
