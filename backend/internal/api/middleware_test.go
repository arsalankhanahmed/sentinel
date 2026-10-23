package api

import (
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/golang-jwt/jwt/v5"
)

const secret = "unit-test-secret"

func token(t *testing.T, role string, exp time.Duration, method jwt.SigningMethod, key any) string {
	t.Helper()
	s, err := jwt.NewWithClaims(method, jwt.MapClaims{
		"sub": "alice", "role": role, "exp": time.Now().Add(exp).Unix(),
	}).SignedString(key)
	if err != nil {
		t.Fatal(err)
	}
	return s
}

func router(disabled bool) *gin.Engine {
	gin.SetMode(gin.TestMode)
	r := gin.New()
	g := r.Group("/api", securityHeaders(), authenticate(secret, disabled))
	g.GET("/read", func(c *gin.Context) { c.String(200, c.GetString("actor")) })
	g.GET("/admin", requireAdmin(disabled), func(c *gin.Context) { c.String(200, "ok") })
	return r
}

func do(r *gin.Engine, path, tok string) *httptest.ResponseRecorder {
	req := httptest.NewRequest(http.MethodGet, path, nil)
	if tok != "" {
		req.Header.Set("Authorization", "Bearer "+tok)
	}
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)
	return w
}

func TestAuthMatrix(t *testing.T) {
	r := router(false)
	admin := token(t, "admin", time.Hour, jwt.SigningMethodHS256, []byte(secret))
	viewer := token(t, "viewer", time.Hour, jwt.SigningMethodHS256, []byte(secret))
	expired := token(t, "admin", -time.Hour, jwt.SigningMethodHS256, []byte(secret))
	wrongKey := token(t, "admin", time.Hour, jwt.SigningMethodHS256, []byte("other"))
	none := token(t, "admin", time.Hour, jwt.SigningMethodNone, jwt.UnsafeAllowNoneSignatureType)

	cases := []struct {
		name, path, tok string
		want            int
	}{
		{"no token", "/api/read", "", 401},
		{"garbage", "/api/read", "abc.def.ghi", 401},
		{"expired", "/api/read", expired, 401},
		{"wrong key", "/api/read", wrongKey, 401},
		{"alg none", "/api/read", none, 401},
		{"viewer read", "/api/read", viewer, 200},
		{"viewer admin", "/api/admin", viewer, 403},
		{"admin admin", "/api/admin", admin, 200},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			if got := do(r, c.path, c.tok).Code; got != c.want {
				t.Fatalf("got %d want %d", got, c.want)
			}
		})
	}
}

func TestActorFromToken(t *testing.T) {
	w := do(router(false), "/api/read", token(t, "viewer", time.Hour, jwt.SigningMethodHS256, []byte(secret)))
	if w.Body.String() != "alice" {
		t.Fatalf("actor = %q", w.Body.String())
	}
}

func TestSecurityHeaders(t *testing.T) {
	w := do(router(true), "/api/read", "")
	for k, v := range map[string]string{
		"Referrer-Policy": "no-referrer", "X-Frame-Options": "DENY",
		"X-Content-Type-Options": "nosniff", "Cache-Control": "no-store",
	} {
		if w.Header().Get(k) != v {
			t.Errorf("%s = %q", k, w.Header().Get(k))
		}
	}
}

func TestAuthDisabledDevMode(t *testing.T) {
	if do(router(true), "/api/admin", "").Code != 200 {
		t.Fatal("dev mode should bypass auth")
	}
}
