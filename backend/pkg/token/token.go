// Package token 提供最小的 HS256 JWT 签发与校验（不引入第三方 JWT 库）。
package token

import (
	"crypto/hmac"
	"crypto/sha256"
	"crypto/subtle"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"time"
)

var (
	ErrInvalid = errors.New("令牌无效")
	ErrExpired = errors.New("令牌已过期")
)

type Claims struct {
	Subject string `json:"sub"`
	Role    string `json:"role"`
	Name    string `json:"name"`
	Issued  int64  `json:"iat"`
	Expires int64  `json:"exp"`
	ID      string `json:"jti"`
}

type Issuer struct {
	secret []byte
	ttl    time.Duration
	now    func() time.Time
}

func NewIssuer(secret string, ttl time.Duration) (*Issuer, error) {
	if len(secret) < 32 {
		return nil, errors.New("HM_JWT_SECRET 至少 32 字节")
	}
	if ttl <= 0 {
		return nil, errors.New("token TTL 必须大于 0")
	}
	return &Issuer{secret: []byte(secret), ttl: ttl, now: time.Now}, nil
}

// Issue 返回 token 与过期时间。
func (i *Issuer) Issue(subject, role, name, jti string) (string, time.Time, error) {
	now := i.now().UTC()
	exp := now.Add(i.ttl)
	claims := Claims{
		Subject: subject,
		Role:    role,
		Name:    name,
		Issued:  now.Unix(),
		Expires: exp.Unix(),
		ID:      jti,
	}
	header := base64.RawURLEncoding.EncodeToString([]byte(`{"alg":"HS256","typ":"JWT"}`))
	body, err := json.Marshal(claims)
	if err != nil {
		return "", time.Time{}, fmt.Errorf("编码 claims: %w", err)
	}
	payload := base64.RawURLEncoding.EncodeToString(body)
	signing := header + "." + payload
	mac := hmac.New(sha256.New, i.secret)
	_, _ = mac.Write([]byte(signing))
	sig := base64.RawURLEncoding.EncodeToString(mac.Sum(nil))
	return signing + "." + sig, exp, nil
}

func (i *Issuer) Parse(raw string) (*Claims, error) {
	parts := strings.Split(raw, ".")
	if len(parts) != 3 {
		return nil, ErrInvalid
	}
	mac := hmac.New(sha256.New, i.secret)
	_, _ = mac.Write([]byte(parts[0] + "." + parts[1]))
	want := mac.Sum(nil)
	got, err := base64.RawURLEncoding.DecodeString(parts[2])
	if err != nil || subtle.ConstantTimeCompare(want, got) != 1 {
		return nil, ErrInvalid
	}
	body, err := base64.RawURLEncoding.DecodeString(parts[1])
	if err != nil {
		return nil, ErrInvalid
	}
	var claims Claims
	if err := json.Unmarshal(body, &claims); err != nil {
		return nil, ErrInvalid
	}
	if claims.Subject == "" || claims.Role == "" {
		return nil, ErrInvalid
	}
	if i.now().UTC().Unix() >= claims.Expires {
		return nil, ErrExpired
	}
	return &claims, nil
}
