package jwt

import (
	"errors"
	"fmt"
	"time"

	"github.com/golang-jwt/jwt/v5"
	"github.com/google/uuid"
)

// Claims 中只放 uid 与令牌类型，权限每次从 Redis 取，避免令牌过期前权限无法变更。
type Claims struct {
	UID uint64 `json:"uid"`
	Typ string `json:"typ"`
	jwt.RegisteredClaims
}

// TokenPair 登录成功后下发的双令牌。
type TokenPair struct {
	AccessToken      string
	RefreshToken     string
	AccessJTI        string
	RefreshJTI       string
	AccessExpiresIn  int64
	RefreshExpiresIn int64
}

// Manager 负责签发与解析。
type Manager struct {
	secret     []byte
	issuer     string
	accessTTL  time.Duration
	refreshTTL time.Duration
}

func New(secret, issuer string, accessTTL, refreshTTL time.Duration) *Manager {
	return &Manager{secret: []byte(secret), issuer: issuer, accessTTL: accessTTL, refreshTTL: refreshTTL}
}

func (m *Manager) Issuer() string            { return m.issuer }
func (m *Manager) AccessTTL() time.Duration  { return m.accessTTL }
func (m *Manager) RefreshTTL() time.Duration { return m.refreshTTL }

func (m *Manager) sign(uid uint64, typ string, jti string, ttl time.Duration) (string, error) {
	now := time.Now()
	claims := Claims{
		UID: uid,
		Typ: typ,
		RegisteredClaims: jwt.RegisteredClaims{
			ID:        jti,
			Issuer:    m.issuer,
			Subject:   fmt.Sprintf("%d", uid),
			IssuedAt:  jwt.NewNumericDate(now),
			NotBefore: jwt.NewNumericDate(now),
			ExpiresAt: jwt.NewNumericDate(now.Add(ttl)),
		},
	}
	return jwt.NewWithClaims(jwt.SigningMethodHS256, claims).SignedString(m.secret)
}

// Generate 同时签发 access 与 refresh。
func (m *Manager) Generate(uid uint64) (*TokenPair, error) {
	accessJTI := uuid.NewString()
	refreshJTI := uuid.NewString()

	access, err := m.sign(uid, "access", accessJTI, m.accessTTL)
	if err != nil {
		return nil, err
	}
	refresh, err := m.sign(uid, "refresh", refreshJTI, m.refreshTTL)
	if err != nil {
		return nil, err
	}
	return &TokenPair{
		AccessToken:      access,
		RefreshToken:     refresh,
		AccessJTI:        accessJTI,
		RefreshJTI:       refreshJTI,
		AccessExpiresIn:  int64(m.accessTTL.Seconds()),
		RefreshExpiresIn: int64(m.refreshTTL.Seconds()),
	}, nil
}

// Parse 校验签名与令牌类型。
func (m *Manager) Parse(raw, typ string) (*Claims, error) {
	tok, err := jwt.ParseWithClaims(raw, &Claims{}, func(t *jwt.Token) (any, error) {
		if _, ok := t.Method.(*jwt.SigningMethodHMAC); !ok {
			return nil, fmt.Errorf("unexpected signing method: %v", t.Header["alg"])
		}
		return m.secret, nil
	}, jwt.WithIssuer(m.issuer), jwt.WithLeeway(30*time.Second))
	if err != nil {
		return nil, err
	}
	claims, ok := tok.Claims.(*Claims)
	if !ok {
		return nil, errors.New("invalid claims")
	}
	if typ != "" && claims.Typ != typ {
		return nil, fmt.Errorf("token type mismatch: want %s got %s", typ, claims.Typ)
	}
	return claims, nil
}
