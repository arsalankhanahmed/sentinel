package api

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/gin-gonic/gin"

	"sentinel/internal/connectors"
	"sentinel/internal/engine"
	"sentinel/internal/incident"
	"sentinel/internal/store"
)

// ---------------------------------------------------------------- helpers

func idParam(c *gin.Context) (int64, bool) {
	id, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid id"})
		return 0, false
	}
	return id, true
}

func fail(c *gin.Context, err error) {
	if errors.Is(err, store.ErrNotFound) {
		c.JSON(http.StatusNotFound, gin.H{"error": "not found"})
		return
	}
	c.JSON(http.StatusInternalServerError, gin.H{"error": "internal error", "rid": c.GetString("rid")})
}

func bad(c *gin.Context, msg string) { c.JSON(http.StatusBadRequest, gin.H{"error": msg}) }

var validSev = map[string]bool{"info": true, "warning": true, "critical": true}

func (s *Server) audit(c *gin.Context, action, target string) {
	s.store.Audit(context.Background(), c.GetString("actor"), action, target, c.ClientIP())
}

// ---------------------------------------------------------------- stats & incidents

func (s *Server) stats(c *gin.Context) {
	st, err := s.store.Stats(c)
	if err != nil {
		fail(c, err)
		return
	}
	c.JSON(http.StatusOK, st)
}

func (s *Server) listIncidents(c *gin.Context) {
	limit, _ := strconv.Atoi(c.DefaultQuery("limit", "100"))
	if limit <= 0 || limit > 500 {
		limit = 100
	}
	status := c.Query("status")
	if status != "" && status != "open" && status != "acknowledged" && status != "resolved" {
		bad(c, "invalid status")
		return
	}
	out, err := s.store.ListIncidents(c, status, limit)
	if err != nil {
		fail(c, err)
		return
	}
	c.JSON(http.StatusOK, out)
}

// incidentByLink resolves a deep-link token → full incident incl. captured rows.
func (s *Server) incidentByLink(c *gin.Context) {
	tok := c.Param("token")
	if len(tok) < 40 || len(tok) > 64 {
		c.JSON(http.StatusNotFound, gin.H{"error": "not found"})
		return
	}
	inc, err := s.store.GetIncidentByLink(c, incident.HashToken(tok))
	if err != nil {
		fail(c, err) // same 404 for unknown and expired: no oracle
		return
	}
	s.audit(c, "incident.view", inc.ID)
	c.JSON(http.StatusOK, inc)
}

func (s *Server) ackIncident(c *gin.Context)     { s.setStatus(c, "acknowledged") }
func (s *Server) resolveIncident(c *gin.Context) { s.setStatus(c, "resolved") }

func (s *Server) setStatus(c *gin.Context, status string) {
	id := c.Param("id")
	if len(id) != 36 {
		bad(c, "invalid id")
		return
	}
	if err := s.store.SetIncidentStatus(c, id, status, c.GetString("actor")); err != nil {
		fail(c, err)
		return
	}
	s.audit(c, "incident."+status, id)
	c.JSON(http.StatusOK, gin.H{"status": status})
}

// ---------------------------------------------------------------- data sources

type dsReq struct {
	Name             string `json:"name" binding:"required,max=100"`
	Driver           string `json:"driver" binding:"required,oneof=sqlserver postgres mysql oracle"`
	DSN              string `json:"dsn" binding:"required"`
	HeartbeatEnabled *bool  `json:"heartbeat_enabled"`
}

func (s *Server) listDataSources(c *gin.Context) {
	out, err := s.store.ListDataSources(c)
	if err != nil {
		fail(c, err)
		return
	}
	c.JSON(http.StatusOK, out) // DSN is never serialised (json:"-")
}

func (s *Server) createDataSource(c *gin.Context) {
	var r dsReq
	if err := c.ShouldBindJSON(&r); err != nil {
		bad(c, err.Error())
		return
	}
	enc, err := s.vault.Encrypt([]byte(r.DSN))
	if err != nil {
		fail(c, err)
		return
	}
	hb := true
	if r.HeartbeatEnabled != nil {
		hb = *r.HeartbeatEnabled
	}
	id, err := s.store.CreateDataSource(c, store.DataSource{Name: r.Name, Driver: r.Driver, DSNEnc: enc, HeartbeatEnabled: hb})
	if err != nil {
		if strings.Contains(err.Error(), "duplicate") {
			bad(c, "name already exists")
			return
		}
		fail(c, err)
		return
	}
	s.engine.ConfigChanged()
	s.audit(c, "datasource.create", r.Name)
	c.JSON(http.StatusCreated, gin.H{"id": id})
}

func (s *Server) testDataSource(c *gin.Context) {
	id, ok := idParam(c)
	if !ok {
		return
	}
	ds, err := s.store.GetDataSource(c, id)
	if err != nil {
		fail(c, err)
		return
	}
	db, err := s.conns.Get(ds)
	if err == nil {
		ctx, cancel := context.WithTimeout(c, 10*time.Second)
		defer cancel()
		start := time.Now()
		if err = connectors.Ping(ctx, db, ds.Driver); err == nil {
			c.JSON(http.StatusOK, gin.H{"ok": true, "latency_ms": time.Since(start).Milliseconds()})
			return
		}
	}
	c.JSON(http.StatusOK, gin.H{"ok": false, "error": err.Error()})
}

func (s *Server) deleteDataSource(c *gin.Context) {
	id, ok := idParam(c)
	if !ok {
		return
	}
	if err := s.store.DeleteDataSource(c, id); err != nil {
		fail(c, err)
		return
	}
	s.conns.Invalidate(id)
	s.engine.ConfigChanged()
	s.audit(c, "datasource.delete", strconv.FormatInt(id, 10))
	c.Status(http.StatusNoContent)
}

// ---------------------------------------------------------------- rules (Module A)

type ruleReq struct {
	Name             string `json:"name" binding:"required,max=200"`
	DataSourceID     int64  `json:"data_source_id" binding:"required"`
	SQLText          string `json:"sql_text" binding:"required"`
	Schedule         string `json:"schedule" binding:"required"`
	Severity         string `json:"severity"`
	CorrectiveAction string `json:"corrective_action"`
	MaxRows          int    `json:"max_rows"`
	Enabled          *bool  `json:"enabled"`
}

func (r ruleReq) toRule() (store.Rule, error) {
	if err := engine.ValidateSchedule(r.Schedule); err != nil {
		return store.Rule{}, fmt.Errorf("invalid schedule: %v", err)
	}
	if r.Severity == "" {
		r.Severity = "warning"
	}
	if !validSev[r.Severity] {
		return store.Rule{}, errors.New("invalid severity")
	}
	if r.MaxRows <= 0 {
		r.MaxRows = 500
	}
	if r.MaxRows > 10000 {
		r.MaxRows = 10000
	}
	en := true
	if r.Enabled != nil {
		en = *r.Enabled
	}
	return store.Rule{Name: r.Name, DataSourceID: r.DataSourceID, SQLText: r.SQLText, Schedule: r.Schedule,
		Severity: r.Severity, CorrectiveAction: r.CorrectiveAction, MaxRows: r.MaxRows, Enabled: en}, nil
}

func (s *Server) listRules(c *gin.Context) {
	out, err := s.store.ListRules(c)
	if err != nil {
		fail(c, err)
		return
	}
	c.JSON(http.StatusOK, out)
}

func (s *Server) createRule(c *gin.Context) {
	var req ruleReq
	if err := c.ShouldBindJSON(&req); err != nil {
		bad(c, err.Error())
		return
	}
	r, err := req.toRule()
	if err != nil {
		bad(c, err.Error())
		return
	}
	if r.ID, err = s.store.CreateRule(c, r); err != nil {
		fail(c, err)
		return
	}
	if err := s.engine.ScheduleRule(r); err != nil {
		s.log.Error("schedule rule", "err", err)
	}
	s.audit(c, "rule.create", r.Name)
	c.JSON(http.StatusCreated, gin.H{"id": r.ID})
}

func (s *Server) updateRule(c *gin.Context) {
	id, ok := idParam(c)
	if !ok {
		return
	}
	var req ruleReq
	if err := c.ShouldBindJSON(&req); err != nil {
		bad(c, err.Error())
		return
	}
	r, err := req.toRule()
	if err != nil {
		bad(c, err.Error())
		return
	}
	r.ID = id
	if err := s.store.UpdateRule(c, r); err != nil {
		fail(c, err)
		return
	}
	_ = s.engine.ScheduleRule(r) // hot reload
	s.audit(c, "rule.update", strconv.FormatInt(id, 10))
	c.JSON(http.StatusOK, gin.H{"id": id})
}

func (s *Server) deleteRule(c *gin.Context) {
	id, ok := idParam(c)
	if !ok {
		return
	}
	s.engine.UnscheduleRule(id)
	if err := s.store.DeleteRule(c, id); err != nil {
		fail(c, err)
		return
	}
	s.audit(c, "rule.delete", strconv.FormatInt(id, 10))
	c.Status(http.StatusNoContent)
}

func (s *Server) runRuleNow(c *gin.Context) {
	id, ok := idParam(c)
	if !ok {
		return
	}
	if !s.engine.TriggerRule(id) {
		c.JSON(http.StatusConflict, gin.H{"error": "rule is already running or queue is full"})
		return
	}
	c.JSON(http.StatusAccepted, gin.H{"queued": true})
}

// ---------------------------------------------------------------- watchers (Module B)

func (s *Server) listWatchers(c *gin.Context) {
	out, err := s.store.ListWatchers(c, false)
	if err != nil {
		fail(c, err)
		return
	}
	c.JSON(http.StatusOK, out)
}

func (s *Server) createWatcher(c *gin.Context) {
	var w store.LogWatcher
	if err := c.ShouldBindJSON(&w); err != nil {
		bad(c, err.Error())
		return
	}
	if w.SelectColumns == "" {
		w.SelectColumns = "*"
	}
	if !connectors.ValidIdent(w.TableName) || !connectors.ValidIdent(w.IDColumn) || !connectors.ValidColumnList(w.SelectColumns) {
		bad(c, "invalid table / id column / column list")
		return
	}
	if strings.Contains(w.FilterSQL, ";") || strings.Contains(w.FilterSQL, "--") {
		bad(c, "filter_sql may not contain ';' or comments")
		return
	}
	if w.Name == "" || w.DataSourceID == 0 {
		bad(c, "name and data_source_id are required")
		return
	}
	if w.IntervalSec < 5 {
		w.IntervalSec = 30
	}
	if w.BatchLimit <= 0 || w.BatchLimit > 5000 {
		w.BatchLimit = 200
	}
	if !validSev[w.Severity] {
		w.Severity = "critical"
	}
	w.Enabled = true
	id, err := s.store.CreateWatcher(c, w)
	if err != nil {
		fail(c, err)
		return
	}
	s.engine.ConfigChanged()
	s.audit(c, "watcher.create", w.Name)
	c.JSON(http.StatusCreated, gin.H{"id": id})
}

func (s *Server) deleteWatcher(c *gin.Context) {
	id, ok := idParam(c)
	if !ok {
		return
	}
	if err := s.store.DeleteWatcher(c, id); err != nil {
		fail(c, err)
		return
	}
	s.engine.ConfigChanged()
	s.audit(c, "watcher.delete", strconv.FormatInt(id, 10))
	c.Status(http.StatusNoContent)
}

// ---------------------------------------------------------------- probes (Module C)

func (s *Server) listProbes(c *gin.Context) {
	out, err := s.store.ListProbes(c, false)
	if err != nil {
		fail(c, err)
		return
	}
	c.JSON(http.StatusOK, out)
}

func (s *Server) createProbe(c *gin.Context) {
	var p store.Probe
	if err := c.ShouldBindJSON(&p); err != nil {
		bad(c, err.Error())
		return
	}
	switch p.Kind {
	case "tcp":
		if !strings.Contains(p.Target, ":") {
			bad(c, "tcp target must be host:port")
			return
		}
	case "http":
		if !strings.HasPrefix(p.Target, "http://") && !strings.HasPrefix(p.Target, "https://") {
			bad(c, "http target must be an http(s) URL")
			return
		}
	default:
		bad(c, "kind must be tcp or http")
		return
	}
	if p.Name == "" {
		p.Name = p.Target
	}
	if p.ExpectStatus == 0 {
		p.ExpectStatus = 200
	}
	if p.TimeoutMs < 100 {
		p.TimeoutMs = 3000
	}
	if p.IntervalSec < 5 {
		p.IntervalSec = 60
	}
	if !validSev[p.Severity] {
		p.Severity = "critical"
	}
	p.Enabled = true
	id, err := s.store.CreateProbe(c, p)
	if err != nil {
		fail(c, err)
		return
	}
	s.engine.ConfigChanged()
	s.audit(c, "probe.create", p.Target)
	c.JSON(http.StatusCreated, gin.H{"id": id})
}

func (s *Server) deleteProbe(c *gin.Context) {
	id, ok := idParam(c)
	if !ok {
		return
	}
	if err := s.store.DeleteProbe(c, id); err != nil {
		fail(c, err)
		return
	}
	s.engine.ConfigChanged()
	s.audit(c, "probe.delete", strconv.FormatInt(id, 10))
	c.Status(http.StatusNoContent)
}

// ---------------------------------------------------------------- channels

type chanReq struct {
	Name        string          `json:"name" binding:"required"`
	Kind        string          `json:"kind" binding:"required,oneof=teams zoom whatsapp email"`
	Config      json.RawMessage `json:"config" binding:"required"`
	MinSeverity string          `json:"min_severity"`
}

func (s *Server) listChannels(c *gin.Context) {
	out, err := s.store.ListChannels(c, false)
	if err != nil {
		fail(c, err)
		return
	}
	c.JSON(http.StatusOK, out) // config (secrets) never serialised
}

func (s *Server) createChannel(c *gin.Context) {
	var r chanReq
	if err := c.ShouldBindJSON(&r); err != nil {
		bad(c, err.Error())
		return
	}
	if !json.Valid(r.Config) {
		bad(c, "config must be valid JSON")
		return
	}
	if !validSev[r.MinSeverity] {
		r.MinSeverity = "warning"
	}
	enc, err := s.vault.Encrypt(r.Config)
	if err != nil {
		fail(c, err)
		return
	}
	id, err := s.store.CreateChannel(c, store.Channel{Name: r.Name, Kind: r.Kind, ConfigEnc: enc, MinSeverity: r.MinSeverity, Enabled: true})
	if err != nil {
		fail(c, err)
		return
	}
	s.disp.InvalidateCache()
	s.audit(c, "channel.create", r.Name)
	c.JSON(http.StatusCreated, gin.H{"id": id})
}

func (s *Server) testChannel(c *gin.Context) {
	id, ok := idParam(c)
	if !ok {
		return
	}
	chans, err := s.store.ListChannels(c, false)
	if err != nil {
		fail(c, err)
		return
	}
	for _, ch := range chans {
		if ch.ID == id {
			ctx, cancel := context.WithTimeout(c, 20*time.Second)
			defer cancel()
			if err := s.disp.SendTest(ctx, ch); err != nil {
				c.JSON(http.StatusOK, gin.H{"ok": false, "error": err.Error()})
				return
			}
			c.JSON(http.StatusOK, gin.H{"ok": true})
			return
		}
	}
	c.JSON(http.StatusNotFound, gin.H{"error": "not found"})
}

func (s *Server) deleteChannel(c *gin.Context) {
	id, ok := idParam(c)
	if !ok {
		return
	}
	if err := s.store.DeleteChannel(c, id); err != nil {
		fail(c, err)
		return
	}
	s.disp.InvalidateCache()
	s.audit(c, "channel.delete", strconv.FormatInt(id, 10))
	c.Status(http.StatusNoContent)
}
