package main

import (
	"context"
	"log"
	"math"
	"net/http"
	"strconv"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/martinllanos/only-ai-pods/internal/auth"
	"github.com/martinllanos/only-ai-pods/internal/llm"
	"github.com/martinllanos/only-ai-pods/internal/store"
)

// FinOps: Claude token usage per tenant and its estimated cost (SPEC-CORE-69).

// modelPrice is USD per million tokens. Cache writes use the 5-minute rate
// (1.25x input).
type modelPrice struct{ Input, Output, CacheRead, CacheWrite float64 }

// pricesAsOf is the date of the Anthropic list prices below. They are an
// estimate: the invoice from Anthropic is the source of truth.
const pricesAsOf = "2026-10-06"

var modelPrices = map[string]modelPrice{
	"claude-opus-5-5":   {4, 20, 0.20, 5},
	"claude-opus-5":     {5, 25, 0.50, 6.25},
	"claude-opus-4-8":   {5, 25, 0.50, 6.25},
	"claude-sonnet-5-5": {2, 10, 0.20, 2.5},
	"claude-haiku-5-5":  {0.10, 0.50, 0.01, 0.125},
	"claude-fable-5-1":  {10, 50, 0.25, 12.5},
}

const unattributedTenant = "SIN_TENANT"

// recordLLMUsage stores each model call. It runs in the request path, so it
// gets its own short deadline and never fails the request.
func recordLLMUsage(st store.Store) func(context.Context, llm.Usage) {
	return func(ctx context.Context, u llm.Usage) {
		tenant := u.TenantID
		if tenant == "" {
			tenant = unattributedTenant
		}
		ctx, cancel := context.WithTimeout(context.WithoutCancel(ctx), 2*time.Second)
		defer cancel()
		err := st.RecordLLMUsage(ctx, store.LLMUsage{
			TenantID: tenant, Model: u.Model, Purpose: u.Purpose,
			InputTokens: u.InputTokens, OutputTokens: u.OutputTokens,
			CacheReadTokens: u.CacheReadTokens, CacheWriteTokens: u.CacheWriteTokens,
		})
		if err != nil {
			log.Printf("[finops] could not record usage for %s: %v", tenant, err)
		}
	}
}

// estimateCost returns the USD cost of a total, or false for an unknown model.
func estimateCost(t store.LLMUsageTotal) (float64, bool) {
	p, ok := modelPrices[t.Model]
	if !ok {
		return 0, false
	}
	usd := (float64(t.InputTokens)*p.Input + float64(t.OutputTokens)*p.Output +
		float64(t.CacheReadTokens)*p.CacheRead + float64(t.CacheWriteTokens)*p.CacheWrite) / 1e6
	return usd, true
}

type usageModel struct {
	store.LLMUsageTotal
	EstimatedCostUSD *float64 `json:"estimated_cost_usd"` // null when the model has no known price
}

type usageTenant struct {
	TenantID         string       `json:"tenant_id"`
	Requests         int64        `json:"requests"`
	InputTokens      int64        `json:"input_tokens"`
	OutputTokens     int64        `json:"output_tokens"`
	CacheReadTokens  int64        `json:"cache_read_tokens"`
	CacheWriteTokens int64        `json:"cache_write_tokens"`
	EstimatedCostUSD float64      `json:"estimated_cost_usd"`
	UnpricedModels   []string     `json:"unpriced_models"`
	Models           []usageModel `json:"models"`
}

func roundUSD(v float64) float64 { return math.Round(v*10000) / 10000 }

// summarizeUsage groups the per-model totals by tenant and prices them.
func summarizeUsage(totals []store.LLMUsageTotal) ([]usageTenant, float64) {
	out := []usageTenant{}
	var grand float64
	for _, t := range totals {
		if len(out) == 0 || out[len(out)-1].TenantID != t.TenantID {
			out = append(out, usageTenant{TenantID: t.TenantID, UnpricedModels: []string{}, Models: []usageModel{}})
		}
		u := &out[len(out)-1]
		u.Requests += t.Requests
		u.InputTokens += t.InputTokens
		u.OutputTokens += t.OutputTokens
		u.CacheReadTokens += t.CacheReadTokens
		u.CacheWriteTokens += t.CacheWriteTokens
		m := usageModel{LLMUsageTotal: t}
		if usd, ok := estimateCost(t); ok {
			r := roundUSD(usd)
			m.EstimatedCostUSD = &r
			u.EstimatedCostUSD += usd
			grand += usd
		} else {
			u.UnpricedModels = append(u.UnpricedModels, t.Model)
		}
		u.Models = append(u.Models, m)
	}
	for i := range out {
		out[i].EstimatedCostUSD = roundUSD(out[i].EstimatedCostUSD)
	}
	return out, roundUSD(grand)
}

func registerFinOpsRoutes(r *gin.Engine, st store.Store, guard *auth.Guard) {
	// Usage per tenant over the last ?days= (default 30, max 366).
	// platform_admin and auditor see every tenant; tenant_admin its own.
	r.GET("/api/v1/admin/finops/usage", guard.RequireRoles(auth.RolePlatformAdmin, auth.RoleAuditor, auth.RoleTenantAdmin), func(c *gin.Context) {
		days := 30
		if v := c.Query("days"); v != "" {
			n, err := strconv.Atoi(v)
			if err != nil || n < 1 || n > 366 {
				c.JSON(http.StatusBadRequest, gin.H{"error": "days must be between 1 and 366"})
				return
			}
			days = n
		}
		tenantID := c.Query("tenant_id")
		if tenantID == "GLOBAL" {
			tenantID = ""
		}
		if !guard.CrossTenant(c) {
			var ok bool
			if tenantID, ok = guard.ResolveTenant(c, tenantID, "", false); !ok {
				return
			}
		}
		since := time.Now().UTC().AddDate(0, 0, -days)
		totals, err := st.SumLLMUsage(c.Request.Context(), tenantID, since)
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
			return
		}
		tenants, total := summarizeUsage(totals)
		c.JSON(http.StatusOK, gin.H{
			"status": "OK", "days": days, "since": since, "prices_as_of": pricesAsOf,
			"tenants": tenants, "estimated_cost_usd": total,
		})
	})
}
