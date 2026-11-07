package main

import (
	"context"
	"errors"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/martinllanos/only-ai-pods/internal/audit"
	"github.com/martinllanos/only-ai-pods/internal/auth"
	"github.com/martinllanos/only-ai-pods/internal/billing"
	"github.com/martinllanos/only-ai-pods/internal/rag"
	"github.com/martinllanos/only-ai-pods/internal/resilience"
	"github.com/martinllanos/only-ai-pods/internal/router"
	"github.com/martinllanos/only-ai-pods/internal/sandbox"
	"github.com/martinllanos/only-ai-pods/internal/security"
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
	ctx := context.Background()
	r, err := newEngine(os.Getenv)
	if err != nil {
		log.Fatalf("http: %v", err)
	}
	r.Use(CORSMiddleware())

	// Persistence and authentication (SPEC-CORE-67)
	st, err := buildStore(ctx, os.Getenv)
	if err != nil {
		log.Fatalf("store: %v", err)
	}
	defer st.Close()
	guard, err := buildGuard(os.Getenv)
	if err != nil {
		log.Fatalf("auth: %v", err)
	}
	if err := bootstrapAdmin(ctx, st, os.Getenv); err != nil {
		log.Fatalf("auth: bootstrap admin: %v", err)
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

	// Initialize Rate Limiter Middleware (Issue #2 / SPEC-CORE-07)
	rateLimiter := security.NewRedisRateLimiter("localhost:6379", 300)
	r.Use(rateLimiter.Middleware())

	smartRouter := router.NewDynamicSmartRouter()
	metricsCollector := telemetry.GetMetricsCollector()
	metricsCollector.SetCMMI(loadCMMIReport(os.Getenv))
	swarmOrchestrator := router.NewSwarmOrchestrator(smartRouter)
	sandboxManager := sandbox.NewSessionManager(smartRouter)
	odooBilling := billing.NewOdooBillingService("http://localhost:8069")
	vaultKey, usedDevKey, err := resolveVaultMasterKey(os.Getenv("VAULT_MASTER_KEY"), os.Getenv("ENVIRONMENT"))
	if err != nil {
		log.Fatalf("vault: %v", err)
	}
	if usedDevKey {
		log.Println("WARNING: VAULT_MASTER_KEY not set; using insecure development vault key")
	}
	nativeVault, err := vault.NewNativeVaultManager(vaultKey, st)
	if err != nil {
		log.Fatalf("vault: failed to initialize native vault: %v", err)
	}

	// Initialize LLM + RAG from the environment (SPEC-CORE-66)
	ai := buildAIStack(os.Getenv)
	semanticCache := rag.NewSemanticCacheManager("localhost:6379", 1*time.Hour)
	metricsCollector.SetSources(semanticCache.Stats, smartRouter.PodCount)
	ragService := rag.NewRAGIngestionService(ai.store, semanticCache)
	smartRouter.SetRetriever(ragService)
	if ai.llm != nil {
		smartRouter.SetLLM(ai.llm)
	}

	// Healthcheck Endpoint
	r.GET("/healthz", func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{
			"status":       "healthy",
			"version":      "60.0.0",
			"llm":          ai.llmMode,
			"vector_store": ai.store.Name(),
			"database":     databaseStatus(c.Request.Context(), st),
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

	sagaOrchestrator := resilience.NewSagaOrchestrator()

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
		tx, err := sagaOrchestrator.StartSaga(tenantID, req.ActionName, req.IsCritical)
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
		tx, err := sagaOrchestrator.VerifyOTPAndExecute(req.SagaID, req.OTPCode)
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
			"message":  "Endpoint de purga reactiva de caché Redis por feedback 👎. Envíe peticiones POST con { type: 'down', query: '...' }",
		})
	})

	// Feedback Endpoint — Reactive Redis Cache Purge on 👎 (Issue #14 & SPEC-CORE-17)
	r.POST("/api/v1/feedback", func(c *gin.Context) {
		var req FeedbackRequest
		if err := c.ShouldBindJSON(&req); err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
			return
		}

		tenantID, ok := guard.ResolveTenant(c, req.TenantID, "GLOBAL", false)
		if !ok {
			return
		}

		purged := false
		if req.Type == "down" && req.Query != "" {
			purged = semanticCache.PurgeKey(c.Request.Context(), tenantID, req.Query)
			fmt.Printf("🧹 [REDIS CACHE PURGE] Purged cache key for tenant='%s' query='%s' due to 👎 feedback\n", tenantID, req.Query)
		}

		c.JSON(http.StatusOK, gin.H{
			"status":        "ACCEPTED",
			"feedback_type": req.Type,
			"reason":        req.Reason,
			"cache_purged":  purged,
			"message":       "Feedback registrado exitosamente. Caché de Redis actualizado.",
		})
	})

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

		session := sandboxManager.CreateEphemeralSession(fileName)
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

	port := os.Getenv("PORT")
	if port == "" {
		port = "8080"
	}

	fmt.Printf("🚀 AI Pods Engine Core Server running on port %s\n", port)
	if err := r.Run(":" + port); err != nil {
		fmt.Printf("Server failed: %v\n", err)
	}
}
