package connectors

import (
	"math/big"
	"strings"
	"testing"
	"time"
)

func TestWatcherQueryPerDialect(t *testing.T) {
	cases := []struct {
		driver, want string
	}{
		{"sqlserver", "SELECT TOP (50) Id, Level, Message FROM dbo.ErrorLog WITH (NOLOCK) WHERE Id > @p1 AND (Level = 'Error') ORDER BY Id"},
		{"postgres", "SELECT Id, Level, Message FROM dbo.ErrorLog WHERE Id > $1 AND (Level = 'Error') ORDER BY Id LIMIT 50"},
		{"mysql", "SELECT Id, Level, Message FROM dbo.ErrorLog WHERE Id > ? AND (Level = 'Error') ORDER BY Id LIMIT 50"},
		{"oracle", "SELECT Id, Level, Message FROM dbo.ErrorLog WHERE Id > :1 AND (Level = 'Error') ORDER BY Id FETCH FIRST 50 ROWS ONLY"},
	}
	for _, c := range cases {
		t.Run(c.driver, func(t *testing.T) {
			got, err := WatcherQuery(c.driver, "dbo.ErrorLog", "Id", "Level, Message", "Level = 'Error'", 50)
			if err != nil {
				t.Fatal(err)
			}
			if got != c.want {
				t.Fatalf("\n got: %s\nwant: %s", got, c.want)
			}
		})
	}
}

func TestWatcherQueryStarAndNoFilter(t *testing.T) {
	got, err := WatcherQuery("postgres", "app_error_log", "log_id", "*", "", 10)
	if err != nil {
		t.Fatal(err)
	}
	if got != "SELECT * FROM app_error_log WHERE log_id > $1 ORDER BY log_id LIMIT 10" {
		t.Fatalf("unexpected: %s", got)
	}
}

func TestWatcherQueryRejectsInjection(t *testing.T) {
	bad := []struct{ table, id, cols string }{
		{"logs; DROP TABLE users", "id", "*"},
		{"logs", "id) OR (1=1", "*"},
		{"logs", "id", "msg, (SELECT password FROM users)"},
		{"", "id", "*"},
		{"logs--", "id", "*"},
	}
	for _, b := range bad {
		if _, err := WatcherQuery("postgres", b.table, b.id, b.cols, "", 10); err == nil {
			t.Errorf("expected rejection for %+v", b)
		}
	}
}

func TestValidIdent(t *testing.T) {
	ok := []string{"dbo.ErrorLog", "[dbo].[ErrorLog]", `"public"."logs"`, "`app`.`logs`", "LOG$TABLE", "_x"}
	for _, s := range ok {
		if !ValidIdent(s) {
			t.Errorf("expected valid: %s", s)
		}
	}
	for _, s := range []string{"a b", "x;y", "1abc", strings.Repeat("a", 200), "a'b"} {
		if ValidIdent(s) {
			t.Errorf("expected invalid: %q", s)
		}
	}
}

func TestMaxIDQuery(t *testing.T) {
	q, err := MaxIDQuery("app_error_log", "log_id")
	if err != nil || q != "SELECT COALESCE(MAX(log_id), 0) FROM app_error_log" {
		t.Fatalf("got %q %v", q, err)
	}
}

func TestNormalize(t *testing.T) {
	ts := time.Date(2026, 10, 23, 8, 0, 0, 0, time.FixedZone("PKT", 5*3600))
	if got := normalize(ts); got != "2026-10-23T03:00:00Z" {
		t.Errorf("time: %v", got)
	}
	if got := normalize([]byte("hello")); got != "hello" {
		t.Errorf("bytes: %v", got)
	}
	if got := normalize([]byte{0xff, 0xfe}); !strings.HasPrefix(got.(string), "base64:") {
		t.Errorf("binary: %v", got)
	}
	if got := normalize(big.NewInt(42)); got != "42" {
		t.Errorf("bigint: %v", got)
	}
	if normalize(nil) != nil {
		t.Error("nil")
	}
}

func TestAsInt64(t *testing.T) {
	for _, v := range []any{int64(7), int32(7), 7, uint64(7), float64(7), "7", []byte("7")} {
		got, err := AsInt64(v)
		if err != nil || got != 7 {
			t.Errorf("%T: %v %v", v, got, err)
		}
	}
	if _, err := AsInt64(true); err == nil {
		t.Error("bool should fail")
	}
}
