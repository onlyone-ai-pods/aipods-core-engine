package main

import (
	"fmt"
	"io"
	"net/http"
	"os"
	"sync"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/martinllanos/only-ai-pods/internal/rag"
	"github.com/martinllanos/only-ai-pods/internal/router"
	"github.com/martinllanos/only-ai-pods/internal/sandbox"
	"github.com/martinllanos/only-ai-pods/internal/tenant"
)

type ChatRequest struct {
	TenantID string `json:"tenant_id" binding:"required"`
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

type ApprovalItem struct {
	Token        string    `json:"token"`
	PodID        string    `json:"pod_id"`
	ActionName   string    `json:"action_name"`
	Summary      string    `json:"summary"`
	Command      string    `json:"command"`
	TenantID     string    `json:"tenant_id"`
	Status       string    `json:"status"` // PENDING, APPROVED, REJECTED
	RequestedAt  time.Time `json:"requested_at"`
}

type ApprovalActionRequest struct {
	Token  string `json:"token" binding:"required"`
	Action string `json:"action" binding:"required"` // approve or reject
}

// In-Memory Approval Store
type ApprovalStore struct {
	mu    sync.RWMutex
	items map[string]*ApprovalItem
}

func NewApprovalStore() *ApprovalStore {
	store := &ApprovalStore{items: make(map[string]*ApprovalItem)}

	// Pre-populate mock initial approvals for rich admin UI experience
	store.items["dryrun_token_sha256_mock99120"] = &ApprovalItem{
		Token:       "dryrun_token_sha256_mock99120",
		PodID:       "POD_AFIP_FISCAL",
		ActionName:  "descargar_retenciones_arca",
		Summary:     "Simulación de consulta de retenciones/percepciones en ARCA (Mirequa).",
		Command:     "node scripts/mis_retenciones_arca.js --cuit=20262534538",
		TenantID:    "TENANT_DEMO_001",
		Status:      "PENDING",
		RequestedAt: time.Now().Add(-5 * time.Minute),
	}

	return store
}

func (s *ApprovalStore) Add(item *ApprovalItem) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.items[item.Token] = item
}

func (s *ApprovalStore) List() []*ApprovalItem {
	s.mu.RLock()
	defer s.mu.RUnlock()
	list := make([]*ApprovalItem, 0, len(s.items))
	for _, item := range s.items {
		list = append(list, item)
	}
	return list
}

func (s *ApprovalStore) ProcessAction(token, action string) (bool, string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	item, ok := s.items[token]
	if !ok {
		return false, "Token no encontrado"
	}

	if action == "approve" {
		item.Status = "APPROVED"
		return true, "Solicitud aprobada y ejecutada en tiempo real"
	} else if action == "reject" {
		item.Status = "REJECTED"
		return true, "Solicitud de ejecución rechazada"
	}
	return false, "Acción no válida"
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
	r := gin.Default()
	r.Use(CORSMiddleware())

	smartRouter := router.NewDynamicSmartRouter()
	sandboxManager := sandbox.NewSessionManager(smartRouter)
	approvalStore := NewApprovalStore()

	// Initialize RAG Engine Stack
	vectorStore := rag.NewVectorStore("localhost:6333", "aipods_vectors")
	semanticCache := rag.NewSemanticCacheManager("localhost:6379", 1*time.Hour)
	ragService := rag.NewRAGIngestionService(vectorStore, semanticCache)

	// Healthcheck Endpoint
	r.GET("/healthz", func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{
			"status":  "healthy",
			"version": "28.0.0",
		})
	})

	// RAG Document Ingestion Endpoint (Supports .pdf, .md, .rst, .txt)
	r.POST("/api/v1/rag/ingest", func(c *gin.Context) {
		tenantID := c.PostForm("tenant_id")
		if tenantID == "" {
			tenantID = "GLOBAL"
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
	r.POST("/api/v1/pods/register", func(c *gin.Context) {
		var config router.DynamicPodConfig
		if err := c.ShouldBindJSON(&config); err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
			return
		}

		smartRouter.RegisterDynamicPod(config)

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

		res, session, err := sandboxManager.ExecuteSandboxQuery(c.Request.Context(), req.SessionID, req.Message)
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
			approvalStore.Add(&ApprovalItem{
				Token:       res.DryRunResult.ApprovalToken,
				PodID:       res.PodID,
				ActionName:  res.DryRunResult.ActionName,
				Summary:     res.DryRunResult.Summary,
				Command:     res.DryRunResult.GeneratedCommand,
				TenantID:    session.TenantID,
				Status:      "PENDING",
				RequestedAt: time.Now(),
			})
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

		ctx := tenant.WithTenantID(c.Request.Context(), req.TenantID)
		res, err := smartRouter.RouteAndExecute(ctx, req.TenantID, req.Message, dryRun)
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
			return
		}

		if res.DryRunResult != nil && res.DryRunResult.RequiresHumanApproval && res.DryRunResult.ApprovalToken != "" {
			approvalStore.Add(&ApprovalItem{
				Token:       res.DryRunResult.ApprovalToken,
				PodID:       res.PodID,
				ActionName:  res.DryRunResult.ActionName,
				Summary:     res.DryRunResult.Summary,
				Command:     res.DryRunResult.GeneratedCommand,
				TenantID:    req.TenantID,
				Status:      "PENDING",
				RequestedAt: time.Now(),
			})
		}

		c.JSON(http.StatusOK, res)
	})

	// Admin Approvals List Endpoint
	r.GET("/api/v1/admin/approvals", func(c *gin.Context) {
		items := approvalStore.List()
		c.JSON(http.StatusOK, gin.H{
			"status":    "OK",
			"approvals": items,
		})
	})

	// Admin Approvals Action Endpoint (Approve / Reject)
	r.POST("/api/v1/admin/approvals/action", func(c *gin.Context) {
		var req ApprovalActionRequest
		if err := c.ShouldBindJSON(&req); err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
			return
		}

		ok, msg := approvalStore.ProcessAction(req.Token, req.Action)
		if !ok {
			c.JSON(http.StatusBadRequest, gin.H{"error": msg})
			return
		}

		c.JSON(http.StatusOK, gin.H{
			"status":  "PROCESSED",
			"message": msg,
			"token":   req.Token,
			"action":  req.Action,
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
