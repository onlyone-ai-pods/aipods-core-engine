package main

import (
	"context"
	"errors"
	"fmt"
	"io"
	"log"
	"net"
	"net/http"
	"os"
	"os/signal"
	"strings"
	"sync"
	"syscall"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/martinllanos/only-ai-pods/internal/audit"
	"github.com/martinllanos/only-ai-pods/internal/auth"
	"github.com/martinllanos/only-ai-pods/internal/billing"
	"github.com/martinllanos/only-ai-pods/internal/rag"
	"github.com/martinllanos/only-ai-pods/internal/resilience"
	"github.com/martinllanos/only-ai-pods/internal/router"
	"github.com/martinllanos/only-ai-pods/internal/sandbox"
	"github.com/martinllanos/only-ai-pods/internal/store"
	"github.com/martinllanos/only-ai-pods/internal/telemetry"
	"github.com/martinllanos/only-ai-pods/internal/tenant"
	"github.com/martinllanos/only-ai-pods/internal/vault"
)

type StartSagaRequest struct {
	TenantID   string `json:"tenant_id"`
	ActionName string `json:"action_name" binding:"required"`
	IsCritical bool   `json:"is_critical"`
}

type VerifyOTPRequest struct {
	SagaID  string `json:"saga_id" binding:"required"`
	OTPCode string `json:"otp_code" binding:"required"`
}

type SwarmExecuteRequest struct {
	TenantID   string   `json:"tenant_id"`
	Query      string   `json:"query" binding:"required"`
	TargetPods []string `json:"target_pods"`
	DryRun     bool     `json:"dry_run"`
}

type StoreSecretRequest struct {
	TenantID    string `json:"tenant_id"`
	KeyName     string `json:"key_name" binding:"required"`
	SecretValue string `json:"secret_value" binding:"required"`
}

type RevealSecretRequest struct {
	TenantID string `json:"tenant_id"`
	KeyName  string `json:"key_name" binding:"required"`
}

type PaymentWebhookRequest struct {
	TenantID      string  `json:"tenant_id" binding:"required"`
	InvoiceNumber string  `json:"invoice_number" binding:"required"`
	AmountUSD     float64 `json:"amount_usd" binding:"required"`
}

type FeedbackRequest struct {
	MessageID int    `json:"message_id"`
	Type      string `json:"type" binding:"required"` // "up" | "down"
	Reason    string `json:"reason,omitempty"`
	PodID     string `json:"pod_id,omitempty"`
	Query     string `json:"query,omitempty"`
	TenantID  string `json:"tenant_id,omitempty"`
}

type ChatRequest struct {
	TenantID string `json:"tenant_id"`
	Message  string `json:"message" binding:"required"`
	DryRun   *bool  `json:"dry_run,omitempty"`
}

type SandboxSessionRequest struct {
	FileName string `json:"file_name"`
}

type SandboxQueryRequest struct {
	SessionID string `json:"session_id" binding:"required"`
	Message   string `json:"message" binding:"required"`
}

type ApprovalActionRequest struct {
	Token  string `json:"token" binding:"required"`
	Action string `json:"action" binding:"required"` // approve or reject
}

// simulatedExecutionResult is the canned output shown after approving an ARCA
// points-of-sale action. The pods still do not execute against ARCA.
func simulatedExecutionResult(command, summary string) string {
	cmdLower := strings.ToLower(command + " " + summary)
	var resultOutput string

	if strings.Contains(cmdLower, "odoo") {
		resultOutput = `📍 PUNTOS DE VENTA ARCA (FILTRO: Odoo)
--------------------------------------------------------------------------------
PV N° 00007 | Tipo: Factura Electrónica - Odoo Production   | Estado: ACTIVO
--------------------------------------------------------------------------------
Coincidencias encontradas: 1 (Verificado en ARCA/AFIP)`
	} else if strings.Contains(cmdLower, "rece") || strings.Contains(cmdLower, "web service") {
		resultOutput = `📍 PUNTOS DE VENTA ARCA (FILTRO: RECE)
--------------------------------------------------------------------------------
PV N° 00002 | Tipo: RECE para aplicativo y/o Web Services   | Estado: ACTIVO
--------------------------------------------------------------------------------
Coincidencias encontradas: 1 (Verificado en ARCA/AFIP)`
	} else if strings.Contains(cmdLower, "linea") {
		resultOutput = `📍 PUNTOS DE VENTA ARCA (FILTRO: Comprobantes en Línea)
--------------------------------------------------------------------------------
PV N° 00001 | Tipo: Comprobantes en Línea - Mercado Interno | Estado: ACTIVO
--------------------------------------------------------------------------------
Coincidencias encontradas: 1 (Verificado en ARCA/AFIP)`
	} else if strings.Contains(cmdLower, "inactivo") || strings.Contains(cmdLower, "baja") {
		resultOutput = `📍 PUNTOS DE VENTA INACTIVOS / DADOS DE BAJA EN ARCA
--------------------------------------------------------------------------------
PV N° 00003 | Tipo: FactuWeb Histórico (Deprecado 2021)    | Estado: DADO DE BAJA
PV N° 00005 | Tipo: Controlador Fiscal Sucursal Belgrano   | Estado: INACTIVO
--------------------------------------------------------------------------------
Coincidencias encontradas: 2 (Verificado en ARCA/AFIP)`
	} else if strings.Contains(cmdLower, "todos") {
		resultOutput = `📍 TODOS LOS PUNTOS DE VENTA REGISTRADOS EN ARCA
--------------------------------------------------------------------------------
PV N° 00001 | Tipo: Comprobantes en Línea - Mercado Interno | Estado: ACTIVO
PV N° 00002 | Tipo: RECE para aplicativo y/o Web Services   | Estado: ACTIVO
PV N° 00003 | Tipo: FactuWeb Histórico (Deprecado 2021)    | Estado: DADO DE BAJA
PV N° 00005 | Tipo: Controlador Fiscal Sucursal Belgrano   | Estado: INACTIVO
PV N° 00007 | Tipo: Factura Electrónica - Odoo Production   | Estado: ACTIVO
--------------------------------------------------------------------------------
Total Puntos de Venta Registrados: 5 (Verificado en ARCA/AFIP)`
	} else {
		resultOutput = `📍 PUNTOS DE VENTA ACTIVOS EN ARCA (CUIT 20262534538)
--------------------------------------------------------------------------------
PV N° 00001 | Tipo: Comprobantes en Línea - Mercado Interno | Estado: ACTIVO
PV N° 00002 | Tipo: RECE para aplicativo y/o Web Services   | Estado: ACTIVO
PV N° 00007 | Tipo: Factura Electrónica - Odoo Production   | Estado: ACTIVO
--------------------------------------------------------------------------------
Total Puntos de Venta Activos: 3 (Verificado en ARCA/AFIP)`
	}

	return resultOutput
}

func CORSMiddleware() gin.HandlerFunc {
	return func(c *gin.Context) {
		c.Writer.Header().Set("Access-Control-Allow-Origin", "*")
		c.Writer.Header().Set("Access-Control-Allow-Credentials", "true")
		c.Writer.Header().Set("Access-Control-Allow-Headers", "Content-Type, Content-Length, Accept-Encoding, X-CSRF-Token, Authorization, accept, origin, Cache-Control, X-Requested-With")
		c.Writer.Header().Set("Access-Control-Allow-Methods", "POST, OPTIONS, GET, PUT, DELETE")

		if c.Request.Method == "OPTIONS" {
			c.AbortWithStatus(204)
			return
		}
		c.Next()
	}
}

func main() {
	// SIGINT/SIGTERM start the drain sequence (SPEC-CORE-73.R1).
	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()
	if err := run(ctx, stop, os.Getenv); err != nil {
		log.Printf("server: %v", err)
		stop()
		os.Exit(1)
	}
}

// run builds the server and serves until ctx is cancelled. Errors are returned
// instead of log.Fatalf so the deferred closes always run: Redis first, the
// Postgres pool last (SPEC-CORE-73.R1, R7).
func run(ctx context.Context, stopSignals func(), getenv func(string) string) error {
	r, err := newEngine(getenv)
	if err != nil {
		return fmt.Errorf("http: %w", err)
	}

	// Persistence and authentication (SPEC-CORE-67)
	st, err := buildStore(ctx, getenv)
	if err != nil {
		return fmt.Errorf("store: %w", err)
	}
	defer func() {
		st.Close()
		log.Println("shutdown: database pool closed")
	}()

	// Liveness and readiness go before the auth and rate-limit middleware
	// (SPEC-CORE-73.R5).
	color := strings.TrimSpace(getenv("DEPLOY_COLOR"))
	probes := newHealth(st, buildVersion, color)
	registerHealthRoutes(r, probes)
	r.Use(probes.drainMiddleware(), CORSMiddleware())

	guard, err := buildGuard(getenv)
	if err != nil {
		return fmt.Errorf("auth: %w", err)
	}
	if err := bootstrapAdmin(ctx, st, getenv); err != nil {
		return fmt.Errorf("auth: bootstrap admin: %w", err)
	}
	guard.UserState = func(ctx context.Context, userID string) (bool, int, error) {
		u, err := st.GetUser(ctx, userID)
		if errors.Is(err, store.ErrNotFound) {
			return false, 0, nil
		}
		return err == nil && u.Active, u.TokenVersion, err
	}
	auditLog := auditor{st: st, guard: guard}
	r.Use(guard.Authenticate())
	registerAuthRoutes(r, st, guard, auditLog)
	registerUserRoutes(r, st, guard, auditLog)
	registerApprovalStatusRoute(r, st)

	// One Redis client for the semantic cache and the rate limiter, closed
	// once through the cache (SPEC-CORE-73.R7, R8, R12).
	redisBackend, err := openRedis(getenv)
	if err != nil {
		return err
	}
	cacheOwnsRedis := false
	defer func() {
		if redisBackend != nil && !cacheOwnsRedis {
			_ = redisBackend.Close()
		}
	}()

	// Rate limiter shared by both colors through Redis (Issue #2 / SPEC-CORE-07, SPEC-CORE-73.R8)
	rateLimiter := buildRateLimiter(redisBackend)
	r.Use(rateLimiter.Middleware())

	smartRouter := router.NewDynamicSmartRouter()
	// Dynamic pods live in the store; every instance loads them now and
	// rereads them in the background (SPEC-CORE-73.R10).
	if n, err := syncDynamicPods(ctx, st, smartRouter); err != nil {
		return fmt.Errorf("dynamic pods: %w", err)
	} else if n > 0 {
		log.Printf("dynamic pods: %d loaded from the store", n)
	}
	// Background workers stop on SIGTERM, or when run returns for any other
	// reason; the pool closes only after they exit.
	workerCtx, stopWorkers := context.WithCancel(ctx)
	var workers sync.WaitGroup
	defer func() {
		stopWorkers()
		workers.Wait()
	}()
	startDynamicPodSync(workerCtx, &workers, st, smartRouter,
		durationEnv(getenv, "DYNAMIC_PODS_SYNC_INTERVAL", defaultDynamicPodsSyncInterval))
	metricsCollector := telemetry.GetMetricsCollector()
	metricsCollector.SetCMMI(loadCMMIReport(getenv))
	metricsCollector.SetBuildInfo(buildVersion, color)
	swarmOrchestrator := router.NewSwarmOrchestrator(smartRouter)
	// Sandbox sessions shared by both colors through Redis (SPEC-CORE-73.R11).
	sandboxManager := sandbox.NewSessionManager(smartRouter)
	if redisBackend != nil {
		sandboxManager = sandbox.NewSessionManagerWithStore(smartRouter, sandbox.NewRedisSessionStore(redisBackend.Client()))
	}
	odooBilling := billing.NewOdooBillingService("http://localhost:8069")
	vaultKey, usedDevKey, err := resolveVaultMasterKey(getenv("VAULT_MASTER_KEY"), getenv("ENVIRONMENT"))
	if err != nil {
		return fmt.Errorf("vault: %w", err)
	}
	if usedDevKey {
		log.Println("WARNING: VAULT_MASTER_KEY not set; using insecure development vault key")
	}
	nativeVault, err := vault.NewNativeVaultManager(vaultKey, st)
	if err != nil {
		return fmt.Errorf("vault: failed to initialize native vault: %w", err)
	}

	// Initialize LLM + RAG from the environment (SPEC-CORE-66)
	ai := buildAIStack(getenv)
	drain := loadDrainConfig(getenv)
	if ai.llmTimeout > 0 && drain.ShutdownTimeout <= ai.llmTimeout {
		log.Printf("WARNING: SHUTDOWN_TIMEOUT (%s) should be longer than LLM_TIMEOUT (%s), or draining cuts LLM answers", drain.ShutdownTimeout, ai.llmTimeout)
	}
	// Semantic cache shared by tenants and persisted in Redis (SPEC-CORE-71).
	semanticCache, err := buildSemanticCache(getenv, nativeVault, redisBackend)
	if err != nil {
		return err
	}
	cacheOwnsRedis = true
	// Closed before the Postgres pool (deferred later, so it runs earlier).
	defer func() {
		if err := semanticCache.Close(); err != nil {
			log.Printf("shutdown: semantic cache: %v", err)
		} else {
			log.Println("shutdown: semantic cache closed")
		}
	}()
	smartRouter.SetCache(semanticCache)
	metricsCollector.SetSources(semanticCache.Stats, smartRouter.PodCount)
	ragService := rag.NewRAGIngestionService(ai.store, semanticCache)
	// Dual RAG strategy per tenant (SPEC-CORE-70): lexical index in Postgres
	// (memory without DATABASE_URL), vector search only when an embedder exists.
	ragService.SetLexicalIndex(buildLexicalIndex(st))
	if ai.store != nil {
		smartRouter.SetRetriever(ragService)
	}
	smartRouter.SetLexicalRetriever(ragService)
	smartRouter.SetStrategyResolver(storeStrategyResolver(st))
	smartRouter.SetFallbackHook(metricsCollector.RecordRAGFallback)
	if ai.llm != nil {
		smartRouter.SetLLM(ai.llm)
	}
	registerRAGStrategyRoutes(r, st, guard, auditLog)

	// Healthcheck Endpoint: kept for compatibility, liveness semantics. The
	// database state is cached so this public endpoint does not query
	// Postgres on every call (SPEC-CORE-73.R6).
	r.GET("/healthz", func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{
			"status":       "healthy",
			"version":      buildVersion,
			"color":        color,
			"llm":          ai.llmMode,
			"vector_store": ai.vectorStoreName(),
			"embedder":     ai.embedder,
			"lexical_rag":  ragService.LexicalName(),
			"database":     probes.databaseStatus(c.Request.Context()),
			"auth_mode":    guard.Mode,
		})
	})

	// Swarm Protocol Multi-Pod Orchestration Endpoint (Issue #12 / SPEC-CORE-33)
	r.POST("/api/v1/swarm/execute", func(c *gin.Context) {
		var req SwarmExecuteRequest
		if err := c.ShouldBindJSON(&req); err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
			return
		}

		tenantID, ok := guard.ResolveTenant(c, req.TenantID, "TENANT_DEMO_001", false)
		if !ok {
			return
		}

		swarmRes, err := swarmOrchestrator.ExecuteSwarm(c.Request.Context(), tenantID, req.Query, req.TargetPods, req.DryRun)
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
			return
		}

		c.JSON(http.StatusOK, swarmRes)
	})

	sagaOrchestrator := resilience.NewSagaOrchestratorWithStore(st)

	// Saga Pattern Enterprise Resilience Endpoints (Issue #13 / SPEC-CORE-35)
	r.POST("/api/v1/saga/start", func(c *gin.Context) {
		var req StartSagaRequest
		if err := c.ShouldBindJSON(&req); err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
			return
		}
		tenantID, ok := guard.ResolveTenant(c, req.TenantID, "TENANT_DEMO_001", false)
		if !ok {
			return
		}
		tx, err := sagaOrchestrator.StartSaga(c.Request.Context(), tenantID, req.ActionName, req.IsCritical)
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
			return
		}
		c.JSON(http.StatusOK, tx)
	})

	r.POST("/api/v1/saga/verify-otp", func(c *gin.Context) {
		var req VerifyOTPRequest
		if err := c.ShouldBindJSON(&req); err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
			return
		}
		tx, err := sagaOrchestrator.VerifyOTPAndExecute(c.Request.Context(), req.SagaID, req.OTPCode)
		if err != nil {
			c.JSON(http.StatusUnauthorized, gin.H{"error": err.Error(), "transaction": tx})
			return
		}
		c.JSON(http.StatusOK, tx)
	})

	// Native Vault Endpoints (Issue #11 / SPEC-CORE-27)
	vaultRoles := guard.RequireRoles(auth.RoleTenantAdmin, auth.RolePlatformAdmin)
	r.GET("/api/v1/vault/secrets", vaultRoles, func(c *gin.Context) {
		tenantID, ok := guard.ResolveTenant(c, c.Query("tenant_id"), "GLOBAL", false)
		if !ok {
			return
		}
		secrets, err := nativeVault.ListSecrets(c.Request.Context(), tenantID)
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
			return
		}
		c.JSON(http.StatusOK, gin.H{
			"status":  "OK",
			"secrets": secrets,
		})
	})

	r.POST("/api/v1/vault/secrets", vaultRoles, func(c *gin.Context) {
		var req StoreSecretRequest
		if err := c.ShouldBindJSON(&req); err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
			return
		}

		tenantID, ok := guard.ResolveTenant(c, req.TenantID, "GLOBAL", false)
		if !ok {
			return
		}
		item, err := nativeVault.StoreSecret(c.Request.Context(), tenantID, req.KeyName, req.SecretValue)
		auditLog.record(c, tenantID, "vault.store", req.KeyName, outcome(err))
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
			return
		}

		c.JSON(http.StatusOK, gin.H{
			"status":  "STORED_ENCRYPTED",
			"secret":  item,
			"message": "Secreto cifrado exitosamente en reposo con AES-256-GCM (SPEC-CORE-27).",
		})
	})

	r.POST("/api/v1/vault/reveal", vaultRoles, func(c *gin.Context) {
		var req RevealSecretRequest
		if err := c.ShouldBindJSON(&req); err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
			return
		}

		tenantID, ok := guard.ResolveTenant(c, req.TenantID, "GLOBAL", false)
		if !ok {
			return
		}
		plainText, err := nativeVault.RevealSecret(c.Request.Context(), tenantID, req.KeyName)
		auditLog.record(c, tenantID, "vault.reveal", req.KeyName, outcome(err))
		if err != nil {
			c.JSON(http.StatusNotFound, gin.H{"error": err.Error()})
			return
		}

		c.JSON(http.StatusOK, gin.H{
			"status":     "REVEALED_EPHMERAL",
			"key_name":   req.KeyName,
			"plain_text": plainText,
			"message":    "Secreto descifrado efímeramente en memoria RAM.",
		})
	})

	dossierGenerator := audit.NewDossierGenerator()

	// ISO 9001 & SOC 2 Type II Dossier Endpoint (Issue #9 / SPEC-CORE-32)
	adminRoles := guard.RequireRoles(auth.RoleAuditor, auth.RolePlatformAdmin)
	r.GET("/api/v1/admin/audit/dossier", adminRoles, func(c *gin.Context) {
		tenantID := c.Query("tenant_id")
		dossier, err := dossierGenerator.GenerateDossier(tenantID)
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
			return
		}
		c.JSON(http.StatusOK, dossier)
	})

	// Odoo Billing Subscription GET Endpoint (Issue #8 / SPEC-CORE-26)
	r.GET("/api/v1/billing/subscription", func(c *gin.Context) {
		tenantID, ok := guard.ResolveTenant(c, c.Query("tenant_id"), "TENANT_DEMO_001", true)
		if !ok {
			return
		}
		sub, err := odooBilling.GetSubscription(c.Request.Context(), tenantID)
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
			return
		}
		c.JSON(http.StatusOK, sub)
	})

	// Odoo Billing Payment Webhook Endpoint (Issue #8 / SPEC-CORE-26)
	// Unsigned webhook: until Odoo signs it, only platform admins may call it.
	r.POST("/api/v1/billing/webhook", guard.RequireRoles(auth.RolePlatformAdmin), func(c *gin.Context) {
		var req PaymentWebhookRequest
		if err := c.ShouldBindJSON(&req); err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
			return
		}

		sub, err := odooBilling.ProcessPaymentWebhook(c.Request.Context(), req.TenantID, req.InvoiceNumber, req.AmountUSD)
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
			return
		}

		c.JSON(http.StatusOK, gin.H{
			"status":       "PROD_ACTIVE",
			"subscription": sub,
			"message":      "Pago confirmado e inyectado desde Odoo Billing. Pods activos en PROD_ACTIVE.",
		})
	})

	// Prometheus Metrics Exporter Endpoint (Issue #4 / SPEC-CORE-25)
	r.GET("/metrics", metricsCollector.PrometheusExporterHandler)

	// Admin Telemetry JSON Endpoint: the same measured numbers as /metrics (SPEC-CORE-68).
	r.GET("/api/v1/admin/telemetry", adminRoles, func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{"status": "OK", "telemetry": metricsCollector.Snapshot()})
	})

	// Feedback Status GET Endpoint for Browser Inspection
	r.GET("/api/v1/feedback", func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{
			"status":   "ACTIVE",
			"endpoint": "POST /api/v1/feedback",
			"message":  "Feedback de respuestas: 👍 verifica y 👎 purga la respuesta en caché. Envíe POST con { type: 'up'|'down', query: '<pregunta original>' }",
		})
	})

	// Feedback Endpoint: 👍 promotes, 👎 purges the cached answer (SPEC-CORE-71.R4)
	r.POST("/api/v1/feedback", feedbackHandler(guard, semanticCache))

	// RAG Document Ingestion Endpoint (Supports .pdf, .md, .rst, .txt)
	r.POST("/api/v1/rag/ingest", func(c *gin.Context) {
		// GLOBAL documents are visible to every tenant: only platform admins write them when enforced.
		tenantID, ok := guard.ResolveTenant(c, c.PostForm("tenant_id"), "GLOBAL", false)
		if !ok {
			return
		}

		fileHeader, err := c.FormFile("file")
		if err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": "missing file field in form upload"})
			return
		}

		file, err := fileHeader.Open()
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
			return
		}
		defer file.Close()

		rawBytes, err := io.ReadAll(file)
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
			return
		}

		ctx := tenant.WithTenantID(c.Request.Context(), tenantID)
		chunks, err := ragService.IngestDocument(ctx, tenantID, fileHeader.Filename, rawBytes)
		if err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
			return
		}

		c.JSON(http.StatusOK, gin.H{
			"status":       "INGESTED",
			"file_name":    fileHeader.Filename,
			"chunks_count": len(chunks),
			"message":      fmt.Sprintf("Document '%s' sanitized, chunked (%d chunks), and indexed into Qdrant Vector Store.", fileHeader.Filename, len(chunks)),
		})
	})

	// Dynamic Pod Registration Endpoint
	r.POST("/api/v1/pods/register", guard.RequireRoles(auth.RoleTenantAdmin, auth.RolePlatformAdmin), func(c *gin.Context) {
		var config router.DynamicPodConfig
		if err := c.ShouldBindJSON(&config); err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
			return
		}

		podTenant, ok := guard.ResolveTenant(c, config.TenantID, "GLOBAL", false)
		if !ok {
			return
		}
		config.TenantID = podTenant
		if err := st.UpsertDynamicPod(c.Request.Context(), configToPod(config)); err != nil {
			log.Printf("dynamic pods: store: %v", err)
			c.JSON(http.StatusInternalServerError, gin.H{"error": "could not persist the pod"})
			return
		}
		smartRouter.RegisterDynamicPod(config)
		auditLog.record(c, podTenant, "pod.register", config.PodID, "success")

		c.JSON(http.StatusOK, gin.H{
			"status":  "REGISTERED",
			"pod_id":  config.PodID,
			"message": fmt.Sprintf("AI Pod '%s' registered dynamically at runtime without recompiling Go core.", config.Name),
		})
	})

	// Sandbox Endpoint: Create Ephemeral Session
	r.POST("/api/v1/sandbox/sessions", func(c *gin.Context) {
		var req SandboxSessionRequest
		_ = c.ShouldBindJSON(&req)

		fileName := req.FileName
		if fileName == "" {
			fileName = "documento_prueba.pdf"
		}

		session, err := sandboxManager.CreateEphemeralSession(c.Request.Context(), fileName)
		if err != nil {
			log.Printf("sandbox: %v", err)
			c.JSON(http.StatusServiceUnavailable, gin.H{"error": "sandbox temporarily unavailable"})
			return
		}
		c.JSON(http.StatusOK, gin.H{
			"status":  "CREATED",
			"session": session,
			"message": "Ephemeral Sandbox session initialized. You have 3 free test queries.",
		})
	})

	// Sandbox Endpoint: Execute Free Test Query (Limit: 3 queries)
	r.POST("/api/v1/sandbox/query", func(c *gin.Context) {
		var req SandboxQueryRequest
		if err := c.ShouldBindJSON(&req); err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
			return
		}

		started := time.Now()
		res, session, err := sandboxManager.ExecuteSandboxQuery(c.Request.Context(), req.SessionID, req.Message)
		if err == nil && res != nil {
			metricsCollector.RecordRequest(res.PodID, time.Since(started), http.StatusOK)
		}
		if err != nil {
			c.JSON(http.StatusForbidden, gin.H{
				"error":      err.Error(),
				"session":    session,
				"conversion": "Crea tu cuenta gratis en 1 clic para guardar este AI Pod permanentemente",
			})
			return
		}

		// Track DryRun approval item in Admin Store if produced
		if res.DryRunResult != nil && res.DryRunResult.RequiresHumanApproval && res.DryRunResult.ApprovalToken != "" {
			recordApproval(c, st, res, session.TenantID)
		}

		c.JSON(http.StatusOK, gin.H{
			"response": res,
			"session":  session,
		})
	})

	// Main Chat Completions Endpoint
	r.POST("/api/v1/chat/completions", func(c *gin.Context) {
		var req ChatRequest
		if err := c.ShouldBindJSON(&req); err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
			return
		}

		dryRun := true
		if req.DryRun != nil {
			dryRun = *req.DryRun
		}

		tenantID, ok := guard.ResolveTenant(c, req.TenantID, "", false)
		if !ok {
			return
		}
		if tenantID == "" {
			c.JSON(http.StatusBadRequest, gin.H{"error": "tenant_id is required"})
			return
		}
		ctx := tenant.WithTenantID(c.Request.Context(), tenantID)
		started := time.Now()
		res, err := smartRouter.RouteAndExecute(ctx, tenantID, req.Message, dryRun)
		if err != nil {
			metricsCollector.RecordRequest("", time.Since(started), http.StatusInternalServerError)
			c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
			return
		}
		metricsCollector.RecordRequest(res.PodID, time.Since(started), http.StatusOK)

		if res.DryRunResult != nil && res.DryRunResult.RequiresHumanApproval && res.DryRunResult.ApprovalToken != "" {
			recordApproval(c, st, res, tenantID)
		}

		c.JSON(http.StatusOK, res)
	})

	// Admin Approvals List Endpoint
	r.GET("/api/v1/admin/approvals", adminRoles, func(c *gin.Context) {
		items, err := st.ListApprovals(c.Request.Context(), c.Query("tenant_id"))
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
			return
		}
		c.JSON(http.StatusOK, gin.H{
			"status":    "OK",
			"approvals": items,
		})
	})

	// Admin Approvals Action Endpoint (Approve / Reject)
	r.POST("/api/v1/admin/approvals/action", adminRoles, func(c *gin.Context) {
		var req ApprovalActionRequest
		if err := c.ShouldBindJSON(&req); err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
			return
		}

		var status, msg, execResult string
		switch req.Action {
		case "approve":
			current, err := st.GetApproval(c.Request.Context(), req.Token)
			if err != nil {
				c.JSON(http.StatusBadRequest, gin.H{"error": "Token no encontrado"})
				return
			}
			status, msg = "APPROVED", "Solicitud aprobada (ejecución simulada: el pod todavía no ejecuta contra ARCA)"
			execResult = simulatedExecutionResult(current.Command, current.Summary)
		case "reject":
			status, msg = "REJECTED", "Solicitud de ejecución rechazada"
		default:
			c.JSON(http.StatusBadRequest, gin.H{"error": "Acción no válida"})
			return
		}

		item, err := st.DecideApproval(c.Request.Context(), req.Token, status, execResult, guard.Actor(c))
		switch {
		case errors.Is(err, store.ErrNotFound):
			c.JSON(http.StatusBadRequest, gin.H{"error": "Token no encontrado"})
			return
		case errors.Is(err, store.ErrConflict):
			c.JSON(http.StatusConflict, gin.H{"error": "La solicitud ya fue resuelta", "status": item.Status})
			return
		case err != nil:
			c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
			return
		}
		auditLog.record(c, item.TenantID, "approval."+req.Action, req.Token, "success")

		c.JSON(http.StatusOK, gin.H{
			"status":           "PROCESSED",
			"message":          msg,
			"token":            req.Token,
			"action":           req.Action,
			"execution_result": execResult,
		})
	})

	port := getenv("PORT")
	if port == "" {
		port = "8080"
	}
	ln, err := net.Listen("tcp", ":"+port)
	if err != nil {
		return fmt.Errorf("http: listen: %w", err)
	}
	inflightCtx, cancelInflight := context.WithCancel(context.Background())
	defer cancelInflight()
	srv := newHTTPServer(":"+port, r, inflightCtx)

	log.Printf("🚀 AI Pods Engine Core %s (color %q) running on port %s", buildVersion, color, port)
	return serveUntilSignal(ctx, srv, ln, probes, drain, stopSignals, cancelInflight)
}
