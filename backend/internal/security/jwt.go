// Package security 安全模块：JWT 认证。
// 轻量方案：bcrypt 存储密码 + 双 Token（Access 15min / Refresh 7d）。
// Refresh Token 存 Redis 白名单，支持主动吊销（登出）。
package security

import (
	"errors"
	"fmt"
	"time"

	"github.com/golang-jwt/jwt/v5"
)

// Claims JWT 载荷。
type Claims struct {
	UserID   int64  `json:"uid"`
	Role     string `json:"role"`
	Username string `json:"username"`
	jwt.RegisteredClaims
}

// TokenPair 双 Token。
type TokenPair struct {
	AccessToken  string `json:"access_token"`
	RefreshToken string `json:"refresh_token"`
	ExpiresIn    int64  `json:"expires_in"` // access 有效期（秒）
	UserID       int64  `json:"-"`          // 签发对象（白名单存储用，不外泄）
}

// JWTManager JWT 签发与校验。
type JWTManager struct {
	secret     []byte
	accessTTL  time.Duration
	refreshTTL time.Duration
}

// NewJWTManager 构造。
func NewJWTManager(secret string, accessTTL, refreshTTL time.Duration) *JWTManager {
	return &JWTManager{secret: []byte(secret), accessTTL: accessTTL, refreshTTL: refreshTTL}
}

// Generate 签发双 Token。
func (m *JWTManager) Generate(userID int64, role, username string) (*TokenPair, error) {
	now := time.Now()
	access := jwt.NewWithClaims(jwt.SigningMethodHS256, &Claims{
		UserID: userID, Role: role, Username: username,
		RegisteredClaims: jwt.RegisteredClaims{
			ExpiresAt: jwt.NewNumericDate(now.Add(m.accessTTL)),
			IssuedAt:  jwt.NewNumericDate(now),
			Issuer:    "agent",
		},
	})
	accessStr, err := access.SignedString(m.secret)
	if err != nil {
		return nil, fmt.Errorf("sign access token: %w", err)
	}

	refresh := jwt.NewWithClaims(jwt.SigningMethodHS256, &Claims{
		UserID: userID, Role: role, Username: username,
		RegisteredClaims: jwt.RegisteredClaims{
			ExpiresAt: jwt.NewNumericDate(now.Add(m.refreshTTL)),
			IssuedAt:  jwt.NewNumericDate(now),
			Issuer:    "agent:refresh",
		},
	})
	refreshStr, err := refresh.SignedString(m.secret)
	if err != nil {
		return nil, fmt.Errorf("sign refresh token: %w", err)
	}

	return &TokenPair{AccessToken: accessStr, RefreshToken: refreshStr, ExpiresIn: int64(m.accessTTL.Seconds()), UserID: userID}, nil
}

// Parse 解析并校验 Token。isRefresh 区分两类 token 的 issuer。
func (m *JWTManager) Parse(tokenStr string, isRefresh bool) (*Claims, error) {
	claims := &Claims{}
	_, err := jwt.ParseWithClaims(tokenStr, claims, func(t *jwt.Token) (any, error) {
		if _, ok := t.Method.(*jwt.SigningMethodHMAC); !ok {
			return nil, fmt.Errorf("unexpected signing method: %v", t.Header["alg"])
		}
		return m.secret, nil
	})
	if err != nil {
		if errors.Is(err, jwt.ErrTokenExpired) {
			return nil, err // 上层区分过期
		}
		return nil, fmt.Errorf("parse token: %w", err)
	}
	wantIssuer := "agent"
	if isRefresh {
		wantIssuer = "agent:refresh"
	}
	if claims.Issuer != wantIssuer {
		return nil, fmt.Errorf("token type mismatch")
	}
	return claims, nil
}

// AccessTTL / RefreshTTL 供 Redis 白名单设置过期时间。
func (m *JWTManager) AccessTTL() time.Duration  { return m.accessTTL }
func (m *JWTManager) RefreshTTL() time.Duration { return m.refreshTTL }
