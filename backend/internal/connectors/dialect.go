package connectors

import (
	"fmt"
	"regexp"
	"strings"
)

var (
	identRe = regexp.MustCompile(`^[A-Za-z_\[\]"` + "`" + `][A-Za-z0-9_.\[\]"` + "`" + `$#]*$`)
)

// ValidIdent guards table/column names that are interpolated into SQL.
func ValidIdent(s string) bool { return s != "" && len(s) <= 128 && identRe.MatchString(s) }

func ValidColumnList(s string) bool {
	if strings.TrimSpace(s) == "*" {
		return true
	}
	for _, c := range strings.Split(s, ",") {
		if !ValidIdent(strings.TrimSpace(c)) {
			return false
		}
	}
	return true
}

// WatcherQuery builds a dialect-correct incremental "rows after watermark" query.
// The watermark value is always a bound parameter, never interpolated.
func WatcherQuery(driver, table, idCol, cols, filter string, limit int) (string, error) {
	if !ValidIdent(table) || !ValidIdent(idCol) || !ValidColumnList(cols) {
		return "", fmt.Errorf("invalid identifier in watcher definition")
	}
	selectList := cols
	if strings.TrimSpace(cols) != "*" {
		selectList = idCol + ", " + cols // guarantee the id column is present
	}
	where := idCol + " > " + placeholder(driver, 1)
	if f := strings.TrimSpace(filter); f != "" {
		where += " AND (" + f + ")"
	}
	switch driver {
	case "sqlserver":
		return fmt.Sprintf("SELECT TOP (%d) %s FROM %s WITH (NOLOCK) WHERE %s ORDER BY %s", limit, selectList, table, where, idCol), nil
	case "oracle":
		return fmt.Sprintf("SELECT %s FROM %s WHERE %s ORDER BY %s FETCH FIRST %d ROWS ONLY", selectList, table, where, idCol, limit), nil
	default: // postgres, mysql
		return fmt.Sprintf("SELECT %s FROM %s WHERE %s ORDER BY %s LIMIT %d", selectList, table, where, idCol, limit), nil
	}
}

// MaxIDQuery returns the current high-water mark, used to initialise a new watcher
// so we don't flood alerts with historical rows.
func MaxIDQuery(table, idCol string) (string, error) {
	if !ValidIdent(table) || !ValidIdent(idCol) {
		return "", fmt.Errorf("invalid identifier")
	}
	return fmt.Sprintf("SELECT COALESCE(MAX(%s), 0) FROM %s", idCol, table), nil
}

func placeholder(driver string, n int) string {
	switch driver {
	case "sqlserver":
		return fmt.Sprintf("@p%d", n)
	case "postgres":
		return fmt.Sprintf("$%d", n)
	case "oracle":
		return fmt.Sprintf(":%d", n)
	default:
		return "?"
	}
}
