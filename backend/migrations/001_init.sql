-- Sentinel: Zero-Trust Operational Exception & Observability Platform
-- Core schema (PostgreSQL 14+). Idempotent: safe to run on every boot.

CREATE EXTENSION IF NOT EXISTS pgcrypto;

-- ---------------------------------------------------------------------------
-- External data sources (monitored databases). DSN is AES-256-GCM encrypted
-- at the application layer; the plaintext never touches disk or the API.
-- ---------------------------------------------------------------------------
CREATE TABLE IF NOT EXISTS data_sources (
    id                BIGSERIAL PRIMARY KEY,
    name              TEXT        NOT NULL UNIQUE,
    driver            TEXT        NOT NULL CHECK (driver IN ('sqlserver','postgres','mysql','oracle')),
    dsn_enc           BYTEA       NOT NULL,
    heartbeat_enabled BOOLEAN     NOT NULL DEFAULT TRUE,
    created_at        TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at        TIMESTAMPTZ NOT NULL DEFAULT now()
);

-- ---------------------------------------------------------------------------
-- Module A: Business Data Exception rules (rows > 0 == exception)
-- ---------------------------------------------------------------------------
CREATE TABLE IF NOT EXISTS exception_rules (
    id                BIGSERIAL PRIMARY KEY,
    name              TEXT        NOT NULL,
    data_source_id    BIGINT      NOT NULL REFERENCES data_sources(id) ON DELETE CASCADE,
    sql_text          TEXT        NOT NULL,
    schedule          TEXT        NOT NULL,              -- cron spec or @every 5m
    severity          TEXT        NOT NULL DEFAULT 'warning' CHECK (severity IN ('info','warning','critical')),
    corrective_action TEXT        NOT NULL DEFAULT '',
    max_rows          INT         NOT NULL DEFAULT 500 CHECK (max_rows BETWEEN 1 AND 10000),
    enabled           BOOLEAN     NOT NULL DEFAULT TRUE,
    last_run_at       TIMESTAMPTZ,
    last_status       TEXT,
    last_duration_ms  INT,
    created_at        TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at        TIMESTAMPTZ NOT NULL DEFAULT now()
);
CREATE INDEX IF NOT EXISTS ix_rules_enabled ON exception_rules(enabled) WHERE enabled;

-- ---------------------------------------------------------------------------
-- Module B: Zero-code runtime exception log watchers (cursor/watermark based)
-- ---------------------------------------------------------------------------
CREATE TABLE IF NOT EXISTS log_watchers (
    id                BIGSERIAL PRIMARY KEY,
    name              TEXT        NOT NULL,
    data_source_id    BIGINT      NOT NULL REFERENCES data_sources(id) ON DELETE CASCADE,
    table_name        TEXT        NOT NULL,              -- e.g. dbo.ErrorLog
    id_column         TEXT        NOT NULL,              -- monotonically increasing identity column
    select_columns    TEXT        NOT NULL DEFAULT '*',
    filter_sql        TEXT        NOT NULL DEFAULT '',   -- optional extra predicate, e.g. Level = 'Error'
    interval_sec      INT         NOT NULL DEFAULT 30 CHECK (interval_sec >= 5),
    batch_limit       INT         NOT NULL DEFAULT 200 CHECK (batch_limit BETWEEN 1 AND 5000),
    cursor_value      BIGINT      NOT NULL DEFAULT -1,   -- -1 = not initialised (start from current max)
    severity          TEXT        NOT NULL DEFAULT 'critical' CHECK (severity IN ('info','warning','critical')),
    corrective_action TEXT        NOT NULL DEFAULT '',
    enabled           BOOLEAN     NOT NULL DEFAULT TRUE,
    created_at        TIMESTAMPTZ NOT NULL DEFAULT now()
);

-- ---------------------------------------------------------------------------
-- Module C: Infrastructure probes (TCP handshake / HTTP status)
-- ---------------------------------------------------------------------------
CREATE TABLE IF NOT EXISTS probes (
    id                BIGSERIAL PRIMARY KEY,
    name              TEXT        NOT NULL,
    kind              TEXT        NOT NULL CHECK (kind IN ('tcp','http')),
    target            TEXT        NOT NULL,              -- host:port  or  https://host/health
    expect_status     INT         NOT NULL DEFAULT 200,
    timeout_ms        INT         NOT NULL DEFAULT 3000 CHECK (timeout_ms BETWEEN 100 AND 60000),
    interval_sec      INT         NOT NULL DEFAULT 60 CHECK (interval_sec >= 5),
    severity          TEXT        NOT NULL DEFAULT 'critical' CHECK (severity IN ('info','warning','critical')),
    enabled           BOOLEAN     NOT NULL DEFAULT TRUE,
    created_at        TIMESTAMPTZ NOT NULL DEFAULT now()
);

-- ---------------------------------------------------------------------------
-- Notification channels. Config (webhook URLs, tokens) is encrypted.
-- ---------------------------------------------------------------------------
CREATE TABLE IF NOT EXISTS notification_channels (
    id                BIGSERIAL PRIMARY KEY,
    name              TEXT        NOT NULL,
    kind              TEXT        NOT NULL CHECK (kind IN ('teams','zoom','whatsapp','email')),
    config_enc        BYTEA       NOT NULL,
    min_severity      TEXT        NOT NULL DEFAULT 'warning' CHECK (min_severity IN ('info','warning','critical')),
    enabled           BOOLEAN     NOT NULL DEFAULT TRUE,
    created_at        TIMESTAMPTZ NOT NULL DEFAULT now()
);

-- ---------------------------------------------------------------------------
-- Incidents. Raw captured rows live ONLY here (payload), never in notifications.
-- Deep-link token is stored as SHA-256 hash; the plaintext exists only in the link.
-- ---------------------------------------------------------------------------
CREATE TABLE IF NOT EXISTS incidents (
    id                UUID        PRIMARY KEY DEFAULT gen_random_uuid(),
    kind              TEXT        NOT NULL CHECK (kind IN ('business_exception','runtime_exception','db_outage','endpoint_outage')),
    source_type       TEXT        NOT NULL,             -- rule | watcher | datasource | probe
    source_id         BIGINT      NOT NULL,
    source_name       TEXT        NOT NULL,
    title             TEXT        NOT NULL,
    severity          TEXT        NOT NULL CHECK (severity IN ('info','warning','critical')),
    status            TEXT        NOT NULL DEFAULT 'open' CHECK (status IN ('open','acknowledged','resolved')),
    fingerprint       TEXT        NOT NULL,
    row_count         INT         NOT NULL DEFAULT 0,
    truncated         BOOLEAN     NOT NULL DEFAULT FALSE,
    columns           JSONB       NOT NULL DEFAULT '[]'::jsonb,
    payload           JSONB       NOT NULL DEFAULT '[]'::jsonb,
    summary           TEXT        NOT NULL DEFAULT '',
    corrective_action TEXT        NOT NULL DEFAULT '',
    occurrences       INT         NOT NULL DEFAULT 1,
    link_token_hash   BYTEA       NOT NULL UNIQUE,
    link_expires_at   TIMESTAMPTZ NOT NULL,
    opened_at         TIMESTAMPTZ NOT NULL DEFAULT now(),
    last_seen_at      TIMESTAMPTZ NOT NULL DEFAULT now(),
    acknowledged_by   TEXT,
    acknowledged_at   TIMESTAMPTZ,
    resolved_at       TIMESTAMPTZ
);
-- At most ONE active incident per fingerprint => natural de-duplication / no alert storms.
CREATE UNIQUE INDEX IF NOT EXISTS ux_incidents_active_fp ON incidents(fingerprint) WHERE status <> 'resolved';
CREATE INDEX IF NOT EXISTS ix_incidents_opened   ON incidents(opened_at DESC);
CREATE INDEX IF NOT EXISTS ix_incidents_status   ON incidents(status, severity);

CREATE TABLE IF NOT EXISTS notification_log (
    id                BIGSERIAL PRIMARY KEY,
    incident_id       UUID        NOT NULL REFERENCES incidents(id) ON DELETE CASCADE,
    channel_id        BIGINT      REFERENCES notification_channels(id) ON DELETE SET NULL,
    event             TEXT        NOT NULL,             -- opened | resolved
    status            TEXT        NOT NULL,             -- sent | failed
    attempts          INT         NOT NULL,
    error             TEXT,
    created_at        TIMESTAMPTZ NOT NULL DEFAULT now()
);
CREATE INDEX IF NOT EXISTS ix_notiflog_incident ON notification_log(incident_id);

CREATE TABLE IF NOT EXISTS audit_log (
    id                BIGSERIAL PRIMARY KEY,
    actor             TEXT        NOT NULL,
    action            TEXT        NOT NULL,
    target            TEXT        NOT NULL,
    ip                TEXT,
    created_at        TIMESTAMPTZ NOT NULL DEFAULT now()
);
