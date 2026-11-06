// 认证服务：注册 / 登录 / 刷新 / 登出。
package service

import (
	"context"
	"errors"
	"strconv"
	"time"

	"github.com/redis/go-redis/v9"
	"golang.org/x/crypto/bcrypt"
	"gorm.io/gorm"

	"agent/internal/model"
	"agent/internal/pkg/errcode"
	"agent/internal/repo"
	"agent/internal/security"
)

// AuthService 认证业务。
type AuthService struct {
	repos *repo.Repos
	users *repo.UserRepo
	jwt   *security.JWTManager
	rdb   *redis.Client
}

// NewAuthService 构造。
func NewAuthService(repos *repo.Repos, jwt *security.JWTManager, rdb *redis.Client) *AuthService {
	return &AuthService{repos: repos, users: repos.User, jwt: jwt, rdb: rdb}
}

// RegisterInput 注册参数。
type RegisterInput struct {
	Username string `json:"username" vd:"len($)>2"`
	Email    string `json:"email"`
	Password string `json:"password" vd:"len($)>7"`
}

// Register 注册（首个用户自动成为 admin，方便开箱即用）。
func (s *AuthService) Register(ctx context.Context, in *RegisterInput) (*model.User, error) {
	if exists, err := s.users.ExistsBy(ctx, in.Username, in.Email); err != nil {
		return nil, errcode.ErrInternal.WithCause(err)
	} else if exists {
		return nil, errcode.ErrUserExists
	}

	hash, err := bcrypt.GenerateFromPassword([]byte(in.Password), bcrypt.DefaultCost)
	if err != nil {
		return nil, errcode.ErrInternal.WithCause(err)
	}

	role := "user"
	if n, e := s.users.Count(ctx); e == nil && n == 0 {
		role = "admin" // 首个注册用户 = 管理员
	}

	u := &model.User{Username: in.Username, Email: in.Email, PasswordHash: string(hash), Role: role}
	if err := s.users.Create(ctx, u); err != nil {
		return nil, errcode.ErrInternal.WithCause(err)
	}
	return u, nil
}

// Login 登录：校验密码 → 签发双 Token。
func (s *AuthService) Login(ctx context.Context, username, password string) (*security.TokenPair, error) {
	u, err := s.users.GetByUsername(ctx, username)
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, errcode.ErrLoginFailed
		}
		return nil, errcode.ErrInternal.WithCause(err)
	}
	if u.Status != 1 {
		return nil, errcode.ErrUserLocked
	}
	if err := bcrypt.CompareHashAndPassword([]byte(u.PasswordHash), []byte(password)); err != nil {
		return nil, errcode.ErrLoginFailed
	}
	return s.jwt.Generate(u.ID, u.Role, u.Username)
}

// Refresh 刷新 Access Token（Refresh Token 白名单校验）。
func (s *AuthService) Refresh(ctx context.Context, refreshToken string) (*security.TokenPair, error) {
	claims, err := s.jwt.Parse(refreshToken, true)
	if err != nil {
		// 过期与无效统一按"凭证失效"处理（前端引导重新登录）
		return nil, errcode.ErrTokenInvalid
	}
	// 白名单校验：登出过的 refresh token 立即失效
	ok, err := s.rdb.SIsMember(ctx, refreshKey(claims.UserID), refreshToken).Result()
	if err != nil {
		return nil, errcode.ErrInternal.WithCause(err)
	}
	if !ok {
		return nil, errcode.ErrTokenInvalid.WithMsg("refresh token 已失效")
	}
	return s.jwt.Generate(claims.UserID, claims.Role, claims.Username)
}

// Logout 登出：吊销该用户全部 refresh token。
func (s *AuthService) Logout(ctx context.Context, userID int64) error {
	return s.rdb.Del(ctx, refreshKey(userID)).Err()
}

// StoreRefreshToken 签发后把 refresh token 加入白名单（TTL 与 refresh_ttl 一致）。
func (s *AuthService) StoreRefreshToken(ctx context.Context, userID int64, token string, ttl time.Duration) error {
	key := refreshKey(userID)
	pipe := s.rdb.Pipeline()
	pipe.SAdd(ctx, key, token)
	pipe.Expire(ctx, key, ttl)
	_, err := pipe.Exec(ctx)
	return err
}

func refreshKey(userID int64) string {
	return "nx:auth:refresh:" + strconv.FormatInt(userID, 10)
}
