// Package auth issues and verifies tenant-scoped JWTs and enforces roles
// (SPEC-CORE-67).
package auth

import (
	"crypto/rand"
	"crypto/rsa"
	"crypto/x509"
	"encoding/pem"
	"errors"
	"fmt"
	"time"

	"github.com/golang-jwt/jwt/v5"
)

var (
	ErrMissingAuthHeader = errors.New("missing or malformed Authorization header")
	ErrInvalidToken      = errors.New("invalid or expired JWT token")
)

// Roles (SPEC-CORE-67.R3).
const (
	RoleCustomer      = "customer"
	RoleTenantAdmin   = "tenant_admin"
	RoleAuditor       = "auditor"
	RolePlatformAdmin = "platform_admin"
)

// ValidRole reports whether r is a known role.
func ValidRole(r string) bool {
	switch r {
	case RoleCustomer, RoleTenantAdmin, RoleAuditor, RolePlatformAdmin:
		return true
	}
	return false
}

const issuer = "aipods-enterprise-auth"

type Claims struct {
	TenantID string   `json:"tenant_id"`
	UserID   string   `json:"user_id"`
	Email    string   `json:"email"`
	Roles    []string `json:"roles"`
	jwt.RegisteredClaims
}

// HasRole reports whether the claims include any of the given roles.
func (c *Claims) HasRole(roles ...string) bool {
	for _, have := range c.Roles {
		for _, want := range roles {
			if have == want {
				return true
			}
		}
	}
	return false
}

// Issuer signs tokens with an RSA private key and verifies them with its public half.
type Issuer struct {
	key *rsa.PrivateKey
	ttl time.Duration
}

func NewIssuer(key *rsa.PrivateKey, ttl time.Duration) *Issuer {
	if ttl <= 0 {
		ttl = 8 * time.Hour
	}
	return &Issuer{key: key, ttl: ttl}
}

// ParsePrivateKeyPEM accepts PKCS#1 or PKCS#8 RSA keys of at least 2048 bits.
func ParsePrivateKeyPEM(pemData string) (*rsa.PrivateKey, error) {
	block, _ := pem.Decode([]byte(pemData))
	if block == nil {
		return nil, errors.New("auth: AUTH_SIGNING_KEY is not PEM")
	}
	var key *rsa.PrivateKey
	if k, err := x509.ParsePKCS1PrivateKey(block.Bytes); err == nil {
		key = k
	} else if k8, err8 := x509.ParsePKCS8PrivateKey(block.Bytes); err8 == nil {
		rk, ok := k8.(*rsa.PrivateKey)
		if !ok {
			return nil, errors.New("auth: AUTH_SIGNING_KEY must be an RSA key")
		}
		key = rk
	} else {
		return nil, fmt.Errorf("auth: cannot parse AUTH_SIGNING_KEY: %w", err8)
	}
	if key.N.BitLen() < 2048 {
		return nil, errors.New("auth: AUTH_SIGNING_KEY must be at least 2048 bits")
	}
	return key, nil
}

// GenerateDevKey creates an ephemeral key for development (tokens die on restart).
func GenerateDevKey() (*rsa.PrivateKey, error) {
	return rsa.GenerateKey(rand.Reader, 2048)
}

// Issue signs a token for a user.
func (i *Issuer) Issue(userID, email, tenantID string, roles []string) (string, time.Time, error) {
	now := time.Now()
	exp := now.Add(i.ttl)
	claims := Claims{
		TenantID: tenantID,
		UserID:   userID,
		Email:    email,
		Roles:    roles,
		RegisteredClaims: jwt.RegisteredClaims{
			Subject:   userID,
			Issuer:    issuer,
			IssuedAt:  jwt.NewNumericDate(now),
			NotBefore: jwt.NewNumericDate(now),
			ExpiresAt: jwt.NewNumericDate(exp),
		},
	}
	tok, err := jwt.NewWithClaims(jwt.SigningMethodRS256, claims).SignedString(i.key)
	return tok, exp, err
}

// Verify parses and validates a token (RS256 only, issuer and expiry checked).
func (i *Issuer) Verify(tokenStr string) (*Claims, error) {
	claims := &Claims{}
	tok, err := jwt.ParseWithClaims(tokenStr, claims, func(t *jwt.Token) (interface{}, error) {
		return &i.key.PublicKey, nil
	}, jwt.WithValidMethods([]string{"RS256"}), jwt.WithIssuer(issuer), jwt.WithExpirationRequired())
	if err != nil || !tok.Valid || claims.TenantID == "" || claims.UserID == "" {
		return nil, ErrInvalidToken
	}
	return claims, nil
}
