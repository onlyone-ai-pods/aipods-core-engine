package rag

import (
	"context"
	"testing"
	"time"

	"github.com/martinllanos/only-ai-pods/internal/pod"
	"github.com/martinllanos/only-ai-pods/internal/tenant"
)

func TestRAGPipelineAndSemanticCache(t *testing.T) {
	// 1. Test PDF Ingestion Chunking
	ingestor := NewPDFIngestor(10, 2)
	ctx := context.Background()
	sampleText := "AFIP ARCA es el organismo recaudador fiscal en Argentina. Para solicitar un certificado digital se requiere generar una clave privada RSA y un archivo de solicitud CSR mediante OpenSSL. Este proceso garantiza la firma digital inmutable de comprobantes electronicos."

	chunks, err := ingestor.IngestPDFText(ctx, "tenant_acme", "Normativa_AFIP_2026.pdf", sampleText)
	if err != nil || len(chunks) == 0 {
		t.Fatalf("PDF Ingestion failed: %v", err)
	}
	if chunks[0].TenantID != "tenant_acme" {
		t.Errorf("Expected TenantID tenant_acme, got %s", chunks[0].TenantID)
	}

	// 2. Test Vector Store & Tenant Isolation Filter
	vecStore := NewVectorStore("localhost:6333", "aipods_vectors")
	err = vecStore.StoreChunks(ctx, chunks)
	if err != nil {
		t.Fatalf("Vector Store ingest failed: %v", err)
	}

	// Context with tenant_acme -> Must find chunks
	tenantCtx := tenant.WithTenantID(ctx, "tenant_acme")
	results, err := vecStore.SimilaritySearch(tenantCtx, "clave privada OpenSSL", 3)
	if err != nil || len(results) == 0 {
		t.Fatalf("Vector similarity search failed for valid tenant: %v", err)
	}

	// Context with OTHER tenant -> Must return 0 results (Tenant Isolation Invariant)
	otherTenantCtx := tenant.WithTenantID(ctx, "tenant_other_hacker")
	otherResults, err := vecStore.SimilaritySearch(otherTenantCtx, "clave privada OpenSSL", 3)
	if err != nil {
		t.Fatalf("Vector search error for other tenant: %v", err)
	}
	if len(otherResults) != 0 {
		t.Errorf("SECURITY VIOLATION: Tenant tenant_other_hacker accessed tenant_acme vectors!")
	}

	// 3. Test Semantic Cache
	cacheMgr := NewSemanticCacheManager("localhost:6379", 1*time.Hour)
	mockResp := &pod.PodResponse{
		PodID:     "POD_AFIP_FINANCE",
		Answer:    "Para generar la clave privada use OpenSSL.",
		Citations: []string{"Normativa_AFIP_2026.pdf (Pagina 1)"},
		Status:    "SUCCESS",
	}

	cacheMgr.StoreResponse(ctx, "tenant_acme", "como genero mi clave afip?", mockResp)

	// Hit
	cached, hit := cacheMgr.GetCachedResponse(ctx, "tenant_acme", "como genero mi clave afip?")
	if !hit || cached == nil {
		t.Errorf("Expected cache hit, got miss")
	}

	// Miss (other query)
	_, hit2 := cacheMgr.GetCachedResponse(ctx, "tenant_acme", "otra consulta no guardada")
	if hit2 {
		t.Errorf("Expected cache miss for un-cached query")
	}
}
