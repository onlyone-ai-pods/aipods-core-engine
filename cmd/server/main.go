package main

import (
	"fmt"
	"io"
	"net/http"
	"os"
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

func main() {
	r := gin.Default()
	smartRouter := router.NewDynamicSmartRouter()
	sandboxManager := sandbox.NewSessionManager(smartRouter)

	// Initialize RAG Engine Stack
	vectorStore := rag.NewVectorStore("localhost:6333", "aipods_vectors")
	semanticCache := rag.NewSemanticCacheManager("localhost:6379", 1*time.Hour)
	ragService := rag.NewRAGIngestionService(vectorStore, semanticCache)

	// Healthcheck Endpoint
	r.GET("/healthz", func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{
			"status":  "healthy",
			"version": "8.7.0",
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

		c.JSON(http.StatusOK, res)
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
