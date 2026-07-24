package main

import (
	"fmt"
	"net/http"
	"os"

	"github.com/gin-gonic/gin"
	"github.com/martinllanos/only-ai-pods/internal/router"
	"github.com/martinllanos/only-ai-pods/internal/tenant"
)

type ChatRequest struct {
	TenantID string `json:"tenant_id" binding:"required"`
	Message  string `json:"message" binding:"required"`
	DryRun   *bool  `json:"dry_run,omitempty"`
}

func main() {
	r := gin.Default()
	smartRouter := router.NewSmartRouter()

	// Healthcheck Endpoint
	r.GET("/healthz", func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{
			"status":  "healthy",
			"version": "4.1.0",
		})
	})

	// Sprint 1 MVP: Chat Endpoint matching specs/01_architecture_core/01_smart_router_spec.md
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

		// Inject Tenant Context
		ctx := tenant.WithTenantID(c.Request.Context(), req.TenantID)

		// Route and Execute via Smart Router
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

	fmt.Printf("🚀 AI Pods Engine Server running on port %s\n", port)
	if err := r.Run(":" + port); err != nil {
		fmt.Printf("Server failed: %v\n", err)
	}
}
