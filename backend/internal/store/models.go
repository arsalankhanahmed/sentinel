package store

import (
	"encoding/json"
	"time"
)

type DataSource struct {
	ID               int64     `json:"id"`
	Name             string    `json:"name"`
	Driver           string    `json:"driver"`
	DSNEnc           []byte    `json:"-"`
	HeartbeatEnabled bool      `json:"heartbeat_enabled"`
	CreatedAt        time.Time `json:"created_at"`
	UpdatedAt        time.Time `json:"updated_at"`
}

type Rule struct {
	ID               int64      `json:"id"`
	Name             string     `json:"name"`
	DataSourceID     int64      `json:"data_source_id"`
	SQLText          string     `json:"sql_text"`
	Schedule         string     `json:"schedule"`
	Severity         string     `json:"severity"`
	CorrectiveAction string     `json:"corrective_action"`
	MaxRows          int        `json:"max_rows"`
	Enabled          bool       `json:"enabled"`
	LastRunAt        *time.Time `json:"last_run_at"`
	LastStatus       *string    `json:"last_status"`
	LastDurationMs   *int       `json:"last_duration_ms"`
}

type LogWatcher struct {
	ID               int64  `json:"id"`
	Name             string `json:"name"`
	DataSourceID     int64  `json:"data_source_id"`
	TableName        string `json:"table_name"`
	IDColumn         string `json:"id_column"`
	SelectColumns    string `json:"select_columns"`
	FilterSQL        string `json:"filter_sql"`
	IntervalSec      int    `json:"interval_sec"`
	BatchLimit       int    `json:"batch_limit"`
	CursorValue      int64  `json:"cursor_value"`
	Severity         string `json:"severity"`
	CorrectiveAction string `json:"corrective_action"`
	Enabled          bool   `json:"enabled"`
}

type Probe struct {
	ID           int64  `json:"id"`
	Name         string `json:"name"`
	Kind         string `json:"kind"`
	Target       string `json:"target"`
	ExpectStatus int    `json:"expect_status"`
	TimeoutMs    int    `json:"timeout_ms"`
	IntervalSec  int    `json:"interval_sec"`
	Severity     string `json:"severity"`
	Enabled      bool   `json:"enabled"`
}

type Channel struct {
	ID          int64  `json:"id"`
	Name        string `json:"name"`
	Kind        string `json:"kind"`
	ConfigEnc   []byte `json:"-"`
	MinSeverity string `json:"min_severity"`
	Enabled     bool   `json:"enabled"`
}

// NewIncident is what engines hand to the incident service.
type NewIncident struct {
	Kind             string
	SourceType       string
	SourceID         int64
	SourceName       string
	Title            string
	Severity         string
	Fingerprint      string
	Columns          []string
	Rows             []map[string]any
	Truncated        bool
	Summary          string
	CorrectiveAction string
	LinkTokenHash    []byte
	LinkExpiresAt    time.Time
}

type Incident struct {
	ID               string          `json:"id"`
	Kind             string          `json:"kind"`
	SourceType       string          `json:"source_type"`
	SourceID         int64           `json:"source_id"`
	SourceName       string          `json:"source_name"`
	Title            string          `json:"title"`
	Severity         string          `json:"severity"`
	Status           string          `json:"status"`
	RowCount         int             `json:"row_count"`
	Truncated        bool            `json:"truncated"`
	Columns          json.RawMessage `json:"columns,omitempty"`
	Payload          json.RawMessage `json:"payload,omitempty"`
	Summary          string          `json:"summary"`
	CorrectiveAction string          `json:"corrective_action"`
	Occurrences      int             `json:"occurrences"`
	OpenedAt         time.Time       `json:"opened_at"`
	LastSeenAt       time.Time       `json:"last_seen_at"`
	AcknowledgedBy   *string         `json:"acknowledged_by"`
	ResolvedAt       *time.Time      `json:"resolved_at"`
	LinkExpiresAt    time.Time       `json:"link_expires_at"`
}

type Stats struct {
	OpenByKind     map[string]int `json:"open_by_kind"`
	OpenBySeverity map[string]int `json:"open_by_severity"`
	Last24h        int            `json:"last_24h"`
}
