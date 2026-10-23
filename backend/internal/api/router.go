// Package api exposes the REST surface consumed by the React dashboard.
package api

import (
	"log/slog"
	"net/http"
	"os"
	"path/filepath"
	"strings"

	"github.com/gin-gonic/gin"

	"sentinel/internal/config"
	"sentinel/internal/connectors"
	"sentinel/internal/engine"
	"sentinel/internal/notify"
	"sentinel/internal/store"
	"sentinel/internal/vault"
)

type Server struct {
	cfg    config.Config
	store  *store.Store
	vault  *vault.Vault
	conns  *connectors.Pool
	engine *engine.Engine
	disp   *notify.Dispatcher
	log    *slog.Logger
}

func NewServer(cfg config.Config, st *store.Store, v *vault.Vault, cp *connectors.Pool, e *engine.Engine, d *notify.Dispatcher, log *slog.Logger) *Server {
	return &Server{cfg: cfg, store: st, vault: v, conns: cp, engine: e, disp: d, log: log}
}

func (s *Server) Router() *gin.Engine {
	gin.SetMode(gin.ReleaseMode)
	r := gin.New()
	r.Use(gin.Recovery(), requestID(), accessLog(s.log), cors(s.cfg.CORSOrigins))
	_ = r.SetTrustedProxies(nil)

	r.GET("/healthz", func(c *gin.Context) { c.JSON(http.StatusOK, gin.H{"status": "ok"}) })
	r.GET("/readyz", func(c *gin.Context) {
		if err := s.store.Ping(c); err != nil {
			c.JSON(http.StatusServiceUnavailable, gin.H{"status": "db unavailable"})
			return
		}
		c.JSON(http.StatusOK, gin.H{"status": "ready"})
	})

	v1 := r.Group("/api/v1", securityHeaders(), authenticate(s.cfg.JWTSecret, s.cfg.AuthDisabled))
	{
		// Read surface (any authenticated user)
		v1.GET("/stats", s.stats)
		v1.GET("/incidents", s.listIncidents)
		v1.GET("/incidents/link/:token", s.incidentByLink) // deep-link resolution
		v1.POST("/incidents/:id/ack", s.ackIncident)
		v1.POST("/incidents/:id/resolve", s.resolveIncident)

		// Configuration surface (admin only)
		adm := v1.Group("", requireAdmin(s.cfg.AuthDisabled))
		adm.GET("/datasources", s.listDataSources)
		adm.POST("/datasources", s.createDataSource)
		adm.POST("/datasources/:id/test", s.testDataSource)
		adm.DELETE("/datasources/:id", s.deleteDataSource)

		adm.GET("/rules", s.listRules)
		adm.POST("/rules", s.createRule)
		adm.PUT("/rules/:id", s.updateRule)
		adm.DELETE("/rules/:id", s.deleteRule)
		adm.POST("/rules/:id/run", s.runRuleNow)

		adm.GET("/watchers", s.listWatchers)
		adm.POST("/watchers", s.createWatcher)
		adm.DELETE("/watchers/:id", s.deleteWatcher)

		adm.GET("/probes", s.listProbes)
		adm.POST("/probes", s.createProbe)
		adm.DELETE("/probes/:id", s.deleteProbe)

		adm.GET("/channels", s.listChannels)
		adm.POST("/channels", s.createChannel)
		adm.POST("/channels/:id/test", s.testChannel)
		adm.DELETE("/channels/:id", s.deleteChannel)
	}

	// Optional single-binary deployment: serve the built SPA with history fallback.
	if dir := s.cfg.StaticDir; dir != "" {
		r.NoRoute(func(c *gin.Context) {
			if strings.HasPrefix(c.Request.URL.Path, "/api/") {
				c.JSON(http.StatusNotFound, gin.H{"error": "not found"})
				return
			}
			p := filepath.Join(dir, filepath.Clean("/"+c.Request.URL.Path))
			if fi, err := os.Stat(p); err == nil && !fi.IsDir() {
				if strings.Contains(p, "/assets/") {
					c.Header("Cache-Control", "public, max-age=31536000, immutable")
				}
				c.File(p)
				return
			}
			c.Header("Cache-Control", "no-store")
			c.Header("Referrer-Policy", "no-referrer")
			c.File(filepath.Join(dir, "index.html"))
		})
	}
	return r
}
