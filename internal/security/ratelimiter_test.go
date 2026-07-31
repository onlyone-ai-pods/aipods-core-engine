package security

import (
	"testing"
)

func TestRedisRateLimiter_Allow(t *testing.T) {
	limiter := NewRedisRateLimiter("localhost:6379", 3)

	// Primeras 3 peticiones deben ser permitidas
	for i := 0; i < 3; i++ {
		allowed, rem := limiter.Allow("tenant_test")
		if !allowed {
			t.Fatalf("Petición %d debería ser permitida", i+1)
		}
		if rem != 2-i {
			t.Errorf("Remaining inesperado: obt %d, esp %d", rem, 2-i)
		}
	}

	// Cuarta petición debe ser bloqueada (HTTP 429)
	allowed, rem := limiter.Allow("tenant_test")
	if allowed {
		t.Fatalf("Cuarta petición debería ser bloqueada por rate limit")
	}
	if rem != 0 {
		t.Errorf("Remaining debería ser 0, obtuvo %d", rem)
	}
}
