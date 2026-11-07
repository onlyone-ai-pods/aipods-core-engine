package rag

import (
	"context"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/martinllanos/only-ai-pods/internal/pod"
	"github.com/martinllanos/only-ai-pods/internal/tenant"
)

func TestRAGPipelineAndMultiFormatIngestion(t *testing.T) {
	ctx := context.Background()

	// 1. PDF ingestion extracts real text per page (fixture: official ARCA PDF, 4 pages)
	pdfIngestor := NewPDFIngestor(100, 10)
	samplePDF, err := os.ReadFile("testdata/ARCA_Manual_PuntosDeVenta.pdf")
	if err != nil {
		t.Fatal(err)
	}
	chunksPDF, err := pdfIngestor.IngestPDFText(ctx, "GLOBAL", "ARCA_Manual_PuntosDeVenta.pdf", samplePDF)
	if err != nil || len(chunksPDF) == 0 {
		t.Fatalf("PDF Ingestion failed: %v", err)
	}
	if !strings.Contains(strings.ToLower(chunksPDF[0].Content), "punto") || strings.Contains(chunksPDF[0].Content, "endobj") {
		t.Errorf("PDF chunk should hold extracted text, got %.120q", chunksPDF[0].Content)
	}
	if last := chunksPDF[len(chunksPDF)-1].PageNumber; last < 2 || last > 4 {
		t.Errorf("page numbers should follow the real pages (1-4), last=%d", last)
	}
	if _, err := pdfIngestor.IngestPDFText(ctx, "GLOBAL", "vacio.pdf", []byte("%PDF-1.7\nnot really a pdf")); err == nil {
		t.Error("a PDF without readable text must be rejected")
	}

	// 2. Test Markdown Ingestion (.md) with Section AST Chunking & Anti-Poisoning
	mdIngestor := NewMarkdownIngestor()
	sampleMD := "# Seccion AFIP\nAFIP ARCA regula la emision de comprobantes electronicos.\n\n## Seccion Clave Privada\nGenerar clave privada RSA de 2048 bits mediante OpenSSL."
	chunksMD, err := mdIngestor.IngestMarkdownText(ctx, "tenant_acme", "Manual_AFIP.md", sampleMD)
	if err != nil || len(chunksMD) < 2 {
		t.Fatalf("Markdown Ingestion failed, expected at least 2 section chunks: %v", err)
	}
	if chunksMD[0].Metadata["doc_type"] != "markdown" {
		t.Errorf("Expected doc_type markdown, got %v", chunksMD[0].Metadata["doc_type"])
	}

	// 3. Test reStructuredText Ingestion (.rst)
	rstIngestor := NewRSTIngestor()
	sampleRST := "Titulo Documento\n==============\nContenido de prueba reST para el motor RAG."
	chunksRST, err := rstIngestor.IngestRSTText(ctx, "tenant_acme", "Documentacion.rst", sampleRST)
	if err != nil || len(chunksRST) == 0 {
		t.Fatalf("reST Ingestion failed: %v", err)
	}
	if chunksRST[0].Metadata["doc_type"] != "restructuredtext" {
		t.Errorf("Expected doc_type restructuredtext, got %v", chunksRST[0].Metadata["doc_type"])
	}

	// 4. Test Plain Text Ingestion (.txt)
	txtIngestor := NewPlainTextIngestor(10, 2)
	sampleTXT := "Instrucciones de soporte tecnico para atencion de tickets de clientes."
	chunksTXT, err := txtIngestor.IngestPlainText(ctx, "tenant_acme", "Soporte.txt", sampleTXT)
	if err != nil || len(chunksTXT) == 0 {
		t.Fatalf("Plain Text Ingestion failed: %v", err)
	}
	if chunksTXT[0].Metadata["doc_type"] != "plaintext" {
		t.Errorf("Expected doc_type plaintext, got %v", chunksTXT[0].Metadata["doc_type"])
	}

	// 5. Test Vector Store & Tenant Isolation Filter
	vecStore := NewVectorStore("localhost:6333", "aipods_vectors")
	_ = vecStore.StoreChunks(ctx, chunksPDF)
	_ = vecStore.StoreChunks(ctx, chunksMD)

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

	// 6. Test Semantic Cache
	cacheMgr := NewSemanticCacheManager("localhost:6379", 1*time.Hour)
	mockResp := &pod.PodResponse{
		PodID:     "POD_AFIP_FINANCE",
		Answer:    "Para generar la clave privada use OpenSSL.",
		Citations: []string{"Normativa_AFIP_2026.pdf (Pagina 1)"},
		Status:    "SUCCESS",
	}

	cacheMgr.StoreResponse(ctx, "tenant_acme", "como genero mi clave afip?", mockResp)
	cached, hit := cacheMgr.GetCachedResponse(ctx, "tenant_acme", "como genero mi clave afip?")
	if !hit || cached == nil {
		t.Errorf("Expected cache hit, got miss")
	}
}
