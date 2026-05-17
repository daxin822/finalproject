package auth

import (
	"bytes"
	"crypto/hmac"
	"crypto/sha256"
	"crypto/subtle"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"strings"
	"time"
)

// Role 与 OpenAPI / 前端契约一致：admin 全量；tenant 仅能操作本租户资源。
const (
	RoleAdmin  = "admin"
	RoleTenant = "tenant"
)

var (
	ErrInvalidToken = errors.New("invalid token")
	ErrExpired      = errors.New("token expired")
)

// Claims 为控制面访问令牌载荷（非标准 JWT，格式见 Sign）。
type Claims struct {
	Sub    string `json:"sub"`
	Role   string `json:"role"`
	Tenant string `json:"tenant,omitempty"`
	Exp    int64  `json:"exp"`
}

// Config 控制鉴权开关与签名密钥。
type Config struct {
	Secret   string
	Disabled bool
}

func LoadConfigFromEnv() *Config {
	dis := strings.TrimSpace(os.Getenv("CP_AUTH_DISABLED")) == "1" ||
		strings.EqualFold(strings.TrimSpace(os.Getenv("CP_AUTH_DISABLED")), "true")
	sec := strings.TrimSpace(os.Getenv("CP_JWT_SECRET"))
	if sec == "" {
		dis = true
	}
	return &Config{Secret: sec, Disabled: dis}
}

func (c *Config) Enabled() bool {
	return c != nil && !c.Disabled
}

// AdminClaims 用于 CP_AUTH_DISABLED 时的开发身份。
func AdminClaims() Claims {
	return Claims{Sub: "dev-admin", Role: RoleAdmin, Tenant: "", Exp: time.Now().Add(24 * time.Hour).Unix()}
}

// VerifyBearer 解析 Authorization: Bearer <token>。
func (c *Config) VerifyBearer(authz string) (Claims, error) {
	if !c.Enabled() {
		return AdminClaims(), nil
	}
	const pfx = "Bearer "
	if !strings.HasPrefix(strings.TrimSpace(authz), pfx) {
		return Claims{}, ErrInvalidToken
	}
	raw := strings.TrimSpace(strings.TrimPrefix(authz, pfx))
	return c.VerifyToken(raw)
}

func (c *Config) VerifyToken(token string) (Claims, error) {
	if c.Secret == "" || len(c.Secret) < 8 {
		return Claims{}, fmt.Errorf("CP_JWT_SECRET too short or empty")
	}
	parts := strings.Split(token, ".")
	if len(parts) != 3 || parts[0] != "cp1" {
		return Claims{}, ErrInvalidToken
	}
	payload, err := base64.RawURLEncoding.DecodeString(parts[1])
	if err != nil {
		return Claims{}, ErrInvalidToken
	}
	sigWant, err := base64.RawURLEncoding.DecodeString(parts[2])
	if err != nil {
		return Claims{}, ErrInvalidToken
	}
	mac := hmac.New(sha256.New, []byte(c.Secret))
	mac.Write(payload)
	if subtle.ConstantTimeCompare(mac.Sum(nil), sigWant) != 1 {
		return Claims{}, ErrInvalidToken
	}
	var cl Claims
	if err := json.Unmarshal(payload, &cl); err != nil {
		return Claims{}, ErrInvalidToken
	}
	if time.Now().Unix() > cl.Exp {
		return Claims{}, ErrExpired
	}
	if cl.Role != RoleAdmin && cl.Role != RoleTenant {
		return Claims{}, ErrInvalidToken
	}
	if cl.Role == RoleTenant && strings.TrimSpace(cl.Tenant) == "" {
		return Claims{}, ErrInvalidToken
	}
	return cl, nil
}

func (c *Config) Sign(cl Claims) (string, error) {
	if c.Secret == "" || len(c.Secret) < 8 {
		return "", fmt.Errorf("CP_JWT_SECRET too short or empty")
	}
	payload, err := json.Marshal(cl)
	if err != nil {
		return "", err
	}
	mac := hmac.New(sha256.New, []byte(c.Secret))
	mac.Write(payload)
	sig := mac.Sum(nil)
	return fmt.Sprintf("cp1.%s.%s",
		base64.RawURLEncoding.EncodeToString(payload),
		base64.RawURLEncoding.EncodeToString(sig),
	), nil
}

// TryLogin 校验演示用户；生产环境应替换为 IdP / 数据库。
func TryLogin(username, password string) (Claims, bool) {
	u := strings.TrimSpace(username)
	p := strings.TrimSpace(password)
	if u == "" || p == "" {
		return Claims{}, false
	}
	adminPass := getenv("CP_ADMIN_PASSWORD", "admin")
	tenantPass := getenv("CP_TENANT1_PASSWORD", "tenant1")
	if u == "admin" && subtle.ConstantTimeCompare([]byte(p), []byte(adminPass)) == 1 {
		return Claims{
			Sub:    "admin",
			Role:   RoleAdmin,
			Tenant: "",
			Exp:    time.Now().Add(24 * time.Hour).Unix(),
		}, true
	}
	if u == "tenant1" && subtle.ConstantTimeCompare([]byte(p), []byte(tenantPass)) == 1 {
		return Claims{
			Sub:    "tenant1",
			Role:   RoleTenant,
			Tenant: "tenant1",
			Exp:    time.Now().Add(24 * time.Hour).Unix(),
		}, true
	}
	return Claims{}, false
}

func getenv(k, def string) string {
	if v := strings.TrimSpace(os.Getenv(k)); v != "" {
		return v
	}
	return def
}

// TenantMatches 校验资源所属租户是否可被当前身份访问。
func (cl Claims) TenantMatches(resourceTenant string) bool {
	if cl.Role == RoleAdmin {
		return true
	}
	return subtle.ConstantTimeCompare([]byte(strings.TrimSpace(cl.Tenant)), []byte(strings.TrimSpace(resourceTenant))) == 1
}

// CanMutateTenant 校验写入请求中的 tenant 字段是否与身份一致（admin 任意）。
func (cl Claims) CanMutateTenant(bodyTenant string) bool {
	if cl.Role == RoleAdmin {
		return true
	}
	return subtle.ConstantTimeCompare([]byte(strings.TrimSpace(cl.Tenant)), []byte(strings.TrimSpace(bodyTenant))) == 1
}

// BearerOK 用于测试：比较 Authorization 头与期望值（不含前缀）。
func BearerOK(authz, token string) bool {
	return bytes.Equal([]byte(strings.TrimSpace(strings.TrimPrefix(authz, "Bearer "))), []byte(strings.TrimSpace(token)))
}
