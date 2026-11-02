package cdkshop

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"time"
)

const tokenAudience = "cdkshop"

type Claims struct {
	Subject  int64    `json:"sub"`
	Role     string   `json:"role"`
	Scope    []string `json:"scope"`
	Issuer   string   `json:"iss"`
	Audience string   `json:"aud"`
	Expires  int64    `json:"exp"`
	Issued   int64    `json:"iat"`
}

func SignToken(secret []byte, userID int64, role string, now time.Time) (string, error) {
	if len(secret) < 32 || userID < 1 || (role != "admin" && role != "user") {
		return "", errors.New("invalid cdkshop token signing input")
	}
	claims := Claims{Subject: userID, Role: role, Scope: []string{"cdkshop"}, Issuer: "sub2api-auth", Audience: tokenAudience, Issued: now.Unix(), Expires: now.Add(5 * time.Minute).Unix()}
	body, err := json.Marshal(claims)
	if err != nil {
		return "", err
	}
	encoded := base64.RawURLEncoding.EncodeToString(body)
	mac := hmac.New(sha256.New, secret)
	_, _ = mac.Write([]byte(encoded))
	return encoded + "." + base64.RawURLEncoding.EncodeToString(mac.Sum(nil)), nil
}

func VerifyToken(secret []byte, token string, now time.Time) (*Claims, error) {
	if len(secret) < 32 || len(token) > 4096 {
		return nil, errors.New("invalid cdkshop token")
	}
	parts := strings.Split(token, ".")
	if len(parts) != 2 {
		return nil, errors.New("invalid cdkshop token")
	}
	sig, err := base64.RawURLEncoding.DecodeString(parts[1])
	if err != nil {
		return nil, errors.New("invalid cdkshop token")
	}
	mac := hmac.New(sha256.New, secret)
	_, _ = mac.Write([]byte(parts[0]))
	if !hmac.Equal(sig, mac.Sum(nil)) {
		return nil, errors.New("invalid cdkshop token")
	}
	body, err := base64.RawURLEncoding.DecodeString(parts[0])
	if err != nil {
		return nil, errors.New("invalid cdkshop token")
	}
	var claims Claims
	if err := json.Unmarshal(body, &claims); err != nil {
		return nil, errors.New("invalid cdkshop token")
	}
	if claims.Subject < 1 || claims.Issuer != "sub2api-auth" || claims.Audience != tokenAudience || claims.Expires <= now.Unix() || claims.Issued > now.Add(time.Minute).Unix() || claims.Expires-claims.Issued > int64(5*time.Minute/time.Second) || claims.Role != "admin" && claims.Role != "user" {
		return nil, fmt.Errorf("invalid cdkshop token claims")
	}
	allowed := false
	for _, scope := range claims.Scope {
		if scope == tokenAudience {
			allowed = true
		}
	}
	if !allowed {
		return nil, errors.New("invalid cdkshop token scope")
	}
	return &claims, nil
}
