package auth

import (
	"crypto/rsa"
	"errors"
	"net/http"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/golang-jwt/jwt/v5"
	"github.com/martinllanos/only-ai-pods/internal/tenant"
)

var (
	ErrMissingAuthHeader = errors.New("missing or malformed Authorization header")
	ErrInvalidToken      = errors.New("invalid or expired JWT token")
)

type Claims struct {
	TenantID string   `json:"tenant_id"`
	UserID   string   `json:"user_id"`
	Roles    []string `json:"roles"`
	jwt.RegisteredClaims
}

type JWTAuthenticator struct {
	publicKey *rsa.PublicKey
}

func NewJWTAuthenticator(publicKey *rsa.PublicKey) *JWTAuthenticator {
	return &JWTAuthenticator{publicKey: publicKey}
}

// GenerateMockToken creates a signed RS256 token for testing/dev
func GenerateMockToken(tenantID, userID string, privateKey *rsa.PrivateKey) (string, error) {
	claims := Claims{
		TenantID: tenantID,
		UserID:   userID,
		Roles:    []string{"admin"},
		RegisteredClaims: jwt.RegisteredClaims{
			ExpiresAt: jwt.NewNumericDate(time.Now().Add(24 * time.Hour)),
			IssuedAt:  jwt.NewNumericDate(time.Now()),
			Issuer:    "aipods-enterprise-auth",
		},
	}

	token := jwt.NewWithClaims(jwt.SigningMethodRS256, claims)
	return token.SignedString(privateKey)
}

// Middleware injects tenant_id and validates Bearer token
func (a *JWTAuthenticator) Middleware() gin.HandlerFunc {
	return func(c *gin.Context) {
		authHeader := c.GetHeader("Authorization")
		if authHeader == "" || !strings.HasPrefix(authHeader, "Bearer ") {
			c.JSON(http.StatusUnauthorized, gin.H{"error": ErrMissingAuthHeader.Error()})
			c.Abort()
			return
		}

		tokenStr := strings.TrimPrefix(authHeader, "Bearer ")
		token, err := jwt.ParseWithClaims(tokenStr, &Claims{}, func(token *jwt.Token) (interface{}, error) {
			if _, ok := token.Method.(*jwt.SigningMethodRSA); !ok {
				return nil, ErrInvalidToken
			}
			return a.publicKey, nil
		})

		if err != nil || !token.Valid {
			c.JSON(http.StatusUnauthorized, gin.H{"error": ErrInvalidToken.Error()})
			c.Abort()
			return
		}

		claims, ok := token.Claims.(*Claims)
		if !ok || claims.TenantID == "" {
			c.JSON(http.StatusUnauthorized, gin.H{"error": "invalid tenant claims in token"})
			c.Abort()
			return
		}

		// Inject tenant_id into Gin context & Request Context
		c.Set(string(tenant.TenantIDKey), claims.TenantID)
		ctx := tenant.WithTenantID(c.Request.Context(), claims.TenantID)
		c.Request = c.Request.WithContext(ctx)

		c.Next()
	}
}
