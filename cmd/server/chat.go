package main

import (
	"context"
	"log"
	"net/http"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/martinllanos/only-ai-pods/internal/auth"
	"github.com/martinllanos/only-ai-pods/internal/pod"
	"github.com/martinllanos/only-ai-pods/internal/router"
	"github.com/martinllanos/only-ai-pods/internal/store"
	"github.com/martinllanos/only-ai-pods/internal/telemetry"
	"github.com/martinllanos/only-ai-pods/internal/tenant"
)

// chatRouter is what the chat endpoint needs from the smart router.
type chatRouter interface {
	RouteAndExecute(ctx context.Context, tenantID, query string, dryRun bool) (*pod.PodResponse, error)
}

// registerChatRoute serves POST /api/v1/chat/completions. Only answers with
// an action that needs human approval create a row in the Admin Hub
// approvals (SPEC-CORE-72.R2/R3).
func registerChatRoute(r *gin.Engine, guard *auth.Guard, smartRouter chatRouter, st store.Store, metricsCollector *telemetry.MetricsCollector) {
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
}

// ragTopKFromEnv reads RAG_TOP_K (SPEC-CORE-72.R6): default 10, valid 1 to
// 20; anything else uses 10 and logs a warning.
func ragTopKFromEnv(getenv func(string) string) int {
	n, ok := router.ParseRAGTopK(getenv("RAG_TOP_K"))
	if !ok {
		log.Printf("WARNING: RAG_TOP_K must be an integer between %d and %d; using %d", router.MinRAGTopK, router.MaxRAGTopK, router.DefaultRAGTopK)
	}
	return n
}
