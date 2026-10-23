package connectors

import (
	"database/sql"
	"encoding/base64"
	"fmt"
	"math/big"
	"strconv"
	"time"
	"unicode/utf8"
)

// Result is a dialect-neutral, JSON-ready capture of a result set.
type Result struct {
	Columns   []string         `json:"columns"`
	Rows      []map[string]any `json:"rows"`
	Truncated bool             `json:"truncated"`
}

// ScanRows converts any *sql.Rows into JSON-safe maps, capped at maxRows.
func ScanRows(rows *sql.Rows, maxRows int) (Result, error) {
	cols, err := rows.Columns()
	if err != nil {
		return Result{}, err
	}
	res := Result{Columns: cols, Rows: make([]map[string]any, 0, 16)}
	vals := make([]any, len(cols))
	ptrs := make([]any, len(cols))
	for i := range vals {
		ptrs[i] = &vals[i]
	}
	for rows.Next() {
		if len(res.Rows) >= maxRows {
			res.Truncated = true
			break
		}
		if err := rows.Scan(ptrs...); err != nil {
			return res, err
		}
		m := make(map[string]any, len(cols))
		for i, c := range cols {
			m[c] = normalize(vals[i])
		}
		res.Rows = append(res.Rows, m)
	}
	return res, rows.Err()
}

func normalize(v any) any {
	switch t := v.(type) {
	case nil:
		return nil
	case []byte:
		if utf8.Valid(t) {
			return string(t)
		}
		return "base64:" + base64.StdEncoding.EncodeToString(t)
	case time.Time:
		return t.UTC().Format(time.RFC3339Nano)
	case *big.Int:
		return t.String()
	default:
		return t
	}
}

// AsInt64 coerces a scanned identity column value into int64 (watermark).
func AsInt64(v any) (int64, error) {
	switch t := v.(type) {
	case int64:
		return t, nil
	case int32:
		return int64(t), nil
	case int:
		return int64(t), nil
	case uint64:
		return int64(t), nil
	case float64:
		return int64(t), nil
	case string:
		return strconv.ParseInt(t, 10, 64)
	case []byte:
		return strconv.ParseInt(string(t), 10, 64)
	default:
		return 0, fmt.Errorf("cannot use %T as watermark", v)
	}
}
