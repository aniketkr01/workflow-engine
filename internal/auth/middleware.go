package auth

import (
	"net/http"
	"strings"

	"github.com/aniketkr01/workflow-engine/internal/domain"
	"github.com/gin-gonic/gin"
)

const (
	ClaimsKey = "claims"
)

// Authenticate validates the JWT Bearer token.
func Authenticate(jwtManager *JWTManager) gin.HandlerFunc {
	return func(c *gin.Context) {
		header := c.GetHeader("Authorization")
		if header == "" {
			c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{"error": "missing authorization header"})
			return
		}

		parts := strings.SplitN(header, " ", 2)
		if len(parts) != 2 || !strings.EqualFold(parts[0], "bearer") {
			c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{"error": "invalid authorization header format"})
			return
		}

		claims, err := jwtManager.Verify(parts[1])
		if err != nil {
			c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{"error": err.Error()})
			return
		}

		c.Set(ClaimsKey, claims)
		c.Next()
	}
}

// RequireRole enforces minimum role for an endpoint.
func RequireRole(roles ...domain.Role) gin.HandlerFunc {
	allowed := make(map[domain.Role]bool, len(roles))
	for _, r := range roles {
		allowed[r] = true
	}
	return func(c *gin.Context) {
		claims, ok := c.Get(ClaimsKey)
		if !ok {
			c.AbortWithStatusJSON(http.StatusForbidden, gin.H{"error": "no claims found"})
			return
		}
		userClaims, ok := claims.(*Claims)
		if !ok {
			c.AbortWithStatusJSON(http.StatusForbidden, gin.H{"error": "invalid claims"})
			return
		}
		if !allowed[userClaims.Role] {
			c.AbortWithStatusJSON(http.StatusForbidden, gin.H{"error": "insufficient permissions"})
			return
		}
		c.Next()
	}
}

// GetClaims retrieves claims from gin context.
func GetClaims(c *gin.Context) (*Claims, bool) {
	v, ok := c.Get(ClaimsKey)
	if !ok {
		return nil, false
	}
	claims, ok := v.(*Claims)
	return claims, ok
}
