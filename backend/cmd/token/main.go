// token mints an HS256 API token for local use / service accounts.
//
//	go run ./cmd/token -sub alice@corp.com -role admin -ttl 12h
//
// In production, issue tokens from your IdP (Entra ID / Okta / Keycloak) instead.
package main

import (
	"flag"
	"fmt"
	"os"
	"time"

	"github.com/golang-jwt/jwt/v5"
)

func main() {
	sub := flag.String("sub", "admin", "subject (user id / email)")
	role := flag.String("role", "admin", "role: admin | viewer")
	ttl := flag.Duration("ttl", 12*time.Hour, "token lifetime")
	flag.Parse()

	secret := os.Getenv("JWT_SECRET")
	if secret == "" {
		fmt.Fprintln(os.Stderr, "JWT_SECRET not set")
		os.Exit(1)
	}
	t := jwt.NewWithClaims(jwt.SigningMethodHS256, jwt.MapClaims{
		"sub": *sub, "role": *role, "iat": time.Now().Unix(), "exp": time.Now().Add(*ttl).Unix(),
	})
	s, err := t.SignedString([]byte(secret))
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
	fmt.Println(s)
}
