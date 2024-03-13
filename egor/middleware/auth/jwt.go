package auth

import (
	"context"
	"fmt"
	"net/http"
	"strings"
	"time"

	"github.com/abiiranathan/egor/egor"
	"github.com/golang-jwt/jwt/v5"
)

type claimsType string

const jwtClaimsKey claimsType = "claims"

// JWT creates a JWT middleware with the given secret and options.
func JWT(secret string) egor.Middleware {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
			// Extract the JWT token from the request
			tokenString := req.Header.Get("Authorization")

			// Remove the "Bearer " prefix
			tokenString = strings.TrimPrefix(tokenString, "Bearer ")

			// remove whitespace
			tokenString = strings.TrimSpace(tokenString)

			if tokenString == "" {
				http.Error(w, "Unauthorized", http.StatusUnauthorized)
				return
			}

			// Verify the token
			claims, err := VerifyJWToken(secret, tokenString)
			if err != nil {
				http.Error(w, "Unauthorized", http.StatusUnauthorized)
				return
			}

			ctx := context.WithValue(req.Context(), jwtClaimsKey, claims)
			req = req.WithContext(ctx)

			// Call the next handler if the token is valid
			next.ServeHTTP(w, req)
		})
	}
}

// CreateToken creates a new JWT token with the given payload and expiry duration.
// JWT is signed with the given secret using the HMAC256 alegorithm.
func CreateJWTToken(secret string, payload any, exp time.Duration) (string, error) {
	token := jwt.New(jwt.SigningMethodHS256)
	claims := token.Claims.(jwt.MapClaims)
	claims["payload"] = payload
	claims["exp"] = time.Now().Add(exp).Unix()
	return token.SignedString([]byte(secret))
}

func VerifyJWToken(secret, tokenString string) (jwt.MapClaims, error) {
	token, err := jwt.Parse(tokenString, func(token *jwt.Token) (interface{}, error) {
		// Validate the signing method
		if _, ok := token.Method.(*jwt.SigningMethodHMAC); !ok {
			return nil, fmt.Errorf("unexpected signing method: %v", token.Header["alg"])
		}
		return []byte(secret), nil
	})
	if err != nil {
		return nil, err
	}

	// Validate the token
	if !token.Valid {
		return nil, fmt.Errorf("invalid token")
	}

	return token.Claims.(jwt.MapClaims), nil
}

// GetClaims returns the claims from the request context or nil if not found.
func GetClaims(req *http.Request) jwt.MapClaims {
	claims, ok := req.Context().Value(jwtClaimsKey).(jwt.MapClaims)
	if !ok {
		return nil
	}
	return claims
}
