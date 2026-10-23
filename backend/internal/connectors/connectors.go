// Package connectors manages pooled, lazily-opened connections to EXTERNAL
// monitored databases (SQL Server, PostgreSQL, MySQL, Oracle). No SDK or code is
// ever installed in the target applications — we only read their databases.
package connectors

import (
	"context"
	"database/sql"
	"fmt"
	"sync"
	"time"

	_ "github.com/go-sql-driver/mysql"
	_ "github.com/jackc/pgx/v5/stdlib"
	_ "github.com/microsoft/go-mssqldb"
	_ "github.com/sijms/go-ora/v2"

	"sentinel/internal/store"
	"sentinel/internal/vault"
)

// goDriver maps platform driver names to registered database/sql driver names.
var goDriver = map[string]string{
	"sqlserver": "sqlserver",
	"postgres":  "pgx",
	"mysql":     "mysql",
	"oracle":    "oracle",
}

type entry struct {
	db      *sql.DB
	updated time.Time
}

type Pool struct {
	mu    sync.Mutex
	dbs   map[int64]*entry
	vault *vault.Vault
}

func NewPool(v *vault.Vault) *Pool { return &Pool{dbs: map[int64]*entry{}, vault: v} }

// Get returns a cached *sql.DB for the data source; re-opens if the config changed.
func (p *Pool) Get(ds store.DataSource) (*sql.DB, error) {
	p.mu.Lock()
	defer p.mu.Unlock()
	if e, ok := p.dbs[ds.ID]; ok && e.updated.Equal(ds.UpdatedAt) {
		return e.db, nil
	}
	if e, ok := p.dbs[ds.ID]; ok {
		_ = e.db.Close()
		delete(p.dbs, ds.ID)
	}
	drv, ok := goDriver[ds.Driver]
	if !ok {
		return nil, fmt.Errorf("unsupported driver %q", ds.Driver)
	}
	dsn, err := p.vault.Decrypt(ds.DSNEnc)
	if err != nil {
		return nil, fmt.Errorf("decrypt dsn: %w", err)
	}
	db, err := sql.Open(drv, string(dsn))
	if err != nil {
		return nil, err
	}
	// Small, bounded pools: we are a guest on production databases.
	db.SetMaxOpenConns(4)
	db.SetMaxIdleConns(2)
	db.SetConnMaxLifetime(15 * time.Minute)
	db.SetConnMaxIdleTime(2 * time.Minute)
	p.dbs[ds.ID] = &entry{db: db, updated: ds.UpdatedAt}
	return db, nil
}

func (p *Pool) Invalidate(id int64) {
	p.mu.Lock()
	defer p.mu.Unlock()
	if e, ok := p.dbs[id]; ok {
		_ = e.db.Close()
		delete(p.dbs, id)
	}
}

func (p *Pool) CloseAll() {
	p.mu.Lock()
	defer p.mu.Unlock()
	for id, e := range p.dbs {
		_ = e.db.Close()
		delete(p.dbs, id)
	}
}

// Ping runs the cheapest possible liveness query for the dialect.
func Ping(ctx context.Context, db *sql.DB, driver string) error {
	q := "SELECT 1"
	if driver == "oracle" {
		q = "SELECT 1 FROM DUAL"
	}
	var one int
	return db.QueryRowContext(ctx, q).Scan(&one)
}

// QueryReadOnly executes SQL inside a read-only transaction where the driver
// supports it, and ALWAYS rolls back — nothing a rule executes is ever committed.
// Defence in depth: also give the platform a least-privilege, read-only DB login.
func QueryReadOnly(ctx context.Context, db *sql.DB, query string, args []any, maxRows int) (Result, error) {
	tx, err := db.BeginTx(ctx, &sql.TxOptions{ReadOnly: true})
	if err != nil {
		tx, err = db.BeginTx(ctx, nil) // e.g. SQL Server driver rejects ReadOnly
		if err != nil {
			return Result{}, err
		}
	}
	defer tx.Rollback() //nolint:errcheck
	rows, err := tx.QueryContext(ctx, query, args...)
	if err != nil {
		return Result{}, err
	}
	defer rows.Close()
	return ScanRows(rows, maxRows)
}
