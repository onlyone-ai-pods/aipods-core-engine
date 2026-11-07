package main

import (
	"context"
	"errors"
	"log"
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/martinllanos/only-ai-pods/internal/auth"
	"github.com/martinllanos/only-ai-pods/internal/rag"
	"github.com/martinllanos/only-ai-pods/internal/router"
	"github.com/martinllanos/only-ai-pods/internal/store"
	"github.com/martinllanos/only-ai-pods/internal/tenant"
)

// RAG strategy per tenant (SPEC-CORE-70.R3).

type SetRAGStrategyRequest struct {
	Strategy string `json:"strategy" binding:"required"`
}

// buildLexicalIndex uses Postgres full-text search when the store is Postgres.
func buildLexicalIndex(st store.Store) rag.LexicalIndex {
	if pg, ok := st.(*store.Postgres); ok {
		return rag.NewPostgresLexical(pg.Pool())
	}
	log.Println("WARNING: lexical RAG index lives in memory (no DATABASE_URL); documents are lost on restart")
	return rag.NewMemoryLexical()
}

// storeStrategyResolver reads the tenant's strategy; tenants without a row
// (AUTH_MODE=off demo tenants) or a failing lookup get the default.
func storeStrategyResolver(st store.Store) router.StrategyResolver {
	return func(ctx context.Context, tenantID string) tenant.RAGStrategy {
		s, err := st.GetRAGStrategy(ctx, tenantID)
		if err != nil {
			if !errors.Is(err, store.ErrNotFound) {
				log.Printf("[rag] could not read RAG strategy of tenant %s, using default: %v", tenantID, err)
			}
			return tenant.DefaultRAGStrategy
		}
		return tenant.RAGStrategy(s)
	}
}

// registerRAGStrategyRoutes lets a platform_admin read and change a tenant's
// RAG strategy. Changes are audited.
func registerRAGStrategyRoutes(r *gin.Engine, st store.Store, guard *auth.Guard, audit auditor) {
	admin := guard.RequireRoles(auth.RolePlatformAdmin)

	r.GET("/api/v1/admin/tenants/:id/rag-strategy", admin, func(c *gin.Context) {
		id := c.Param("id")
		s, err := st.GetRAGStrategy(c.Request.Context(), id)
		switch {
		case errors.Is(err, store.ErrNotFound):
			c.JSON(http.StatusNotFound, gin.H{"error": "tenant not found"})
		case err != nil:
			c.JSON(http.StatusInternalServerError, gin.H{"error": "could not read RAG strategy"})
		default:
			c.JSON(http.StatusOK, gin.H{"tenant_id": id, "strategy": s})
		}
	})

	r.PUT("/api/v1/admin/tenants/:id/rag-strategy", admin, func(c *gin.Context) {
		id := c.Param("id")
		var req SetRAGStrategyRequest
		if err := c.ShouldBindJSON(&req); err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
			return
		}
		err := st.SetRAGStrategy(c.Request.Context(), id, req.Strategy)
		audit.record(c, id, "rag_strategy.set", req.Strategy, outcome(err))
		switch {
		case errors.Is(err, store.ErrInvalid):
			c.JSON(http.StatusBadRequest, gin.H{"error": "strategy must be query_expansion or external_embeddings"})
		case errors.Is(err, store.ErrNotFound):
			c.JSON(http.StatusNotFound, gin.H{"error": "tenant not found"})
		case err != nil:
			c.JSON(http.StatusInternalServerError, gin.H{"error": "could not save RAG strategy"})
		default:
			c.JSON(http.StatusOK, gin.H{"tenant_id": id, "strategy": req.Strategy})
		}
	})
}
