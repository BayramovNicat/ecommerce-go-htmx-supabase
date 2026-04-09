package auth

import (
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
)

type UserData struct {
	ID           string                 `json:"id"`
	Email        string                 `json:"email"`
	UserMetadata map[string]interface{} `json:"user_metadata"`
}

type cachedUser struct {
	user      *UserData
	expiresAt time.Time
}

var tokenCache sync.Map

const maxCacheTTL = 5 * time.Minute

// VerifyToken verifies a Supabase JWT and returns the user data.
func VerifyToken(tokenString string) (*UserData, error) {
	if tokenString == "" {
		return nil, errors.New("missing token")
	}

	if user := getCached(tokenString); user != nil {
		return user, nil
	}

	jwtSecret := os.Getenv("SUPABASE_JWT_SECRET")
	if jwtSecret != "" {
		user, exp, err := verifyLocally(tokenString, jwtSecret)
		if err != nil {
			return nil, err
		}
		cache(tokenString, user, exp)
		return user, nil
	}

	user, err := verifyWithAPI(tokenString)
	if err != nil {
		return nil, err
	}

	cache(tokenString, user, time.Now().Add(30*time.Second))
	return user, nil
}

func verifyWithAPI(tokenString string) (*UserData, error) {
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

	var user UserData
	if err := json.NewDecoder(resp.Body).Decode(&user); err != nil {
		return nil, fmt.Errorf("failed to decode supabase user response: %w", err)
	}

	if user.ID == "" {
		return nil, errors.New("supabase user response missing id")
	}

	return &user, nil
}

func verifyLocally(tokenString, jwtSecret string) (*UserData, time.Time, error) {
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

	user := &UserData{
		ID:           sub,
		Email:        claimString(claims, "email"),
		UserMetadata: claimMap(claims, "user_metadata"),
	}

	return user, claimTime(claims, "exp"), nil
}

func getCached(token string) *UserData {
	v, ok := tokenCache.Load(token)
	if !ok {
		return nil
	}
	entry := v.(cachedUser)
	if time.Now().After(entry.expiresAt) {
		tokenCache.Delete(token)
		return nil
	}
	u := *entry.user
	return &u
}

func cache(token string, user *UserData, expiresAt time.Time) {
	if user == nil {
		return
	}
	now := time.Now()
	if expiresAt.IsZero() || expiresAt.Before(now) {
		expiresAt = now.Add(30 * time.Second)
	}
	if max := now.Add(maxCacheTTL); expiresAt.After(max) {
		expiresAt = max
	}
	tokenCache.Store(token, cachedUser{user: user, expiresAt: expiresAt})
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
