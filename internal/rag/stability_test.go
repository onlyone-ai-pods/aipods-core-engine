package rag

import (
	"context"
	"fmt"
	"testing"

	"github.com/martinllanos/only-ai-pods/internal/tenant"
)

const categoriasPDF = "ARCA_Monotributo_Categorias_Feb-Jul2026.pdf"

// arcaCorpus holds representative fragments of the ARCA PDFs. Page 1 of the
// categories PDF is the table: it does not repeat "monotributo" nor
// "categorías". Page 2 has the footnotes, and other manuals talk a lot about
// the Monotributo. Their chunk IDs sort before the table, so ties never favour it.
func arcaCorpus(tableFileName string) []DocumentChunk {
	chunks := []DocumentChunk{
		{ChunkID: "z_cat_p1", FileName: tableFileName, PageNumber: 1, Content: "Vigencia febrero 2026 a julio 2026. " +
			"A ingresos brutos anuales $ 10.277.988,13 superficie 30 m2 energía 3330 kW alquileres $ 2.390.229,80 impuesto integrado $ 4.780,46 aporte SIPA $ 15.616,17 obra social $ 21.990,11 total $ 42.386,74. " +
			"B ingresos brutos $ 15.058.447,71 impuesto integrado $ 9.082,88. C $ 21.113.696,52. D $ 26.212.853,42. E $ 30.833.964,37. " +
			"F $ 38.642.048,36. G $ 46.211.109,37. H $ 70.113.407,33. I $ 78.479.211,62. J $ 89.872.640,30. K $ 108.357.084,05 venta de bienes."},
		{ChunkID: "z_cat_p2", FileName: tableFileName, PageNumber: 2, Content: "(1) Las categorías I, J y K del Monotributo solo corresponden a la venta de cosas muebles. " +
			"(2) Los montos se actualizan semestralmente. (3) Monotributo social y trabajadores independientes promovidos."},
	}
	for i, text := range []string{
		"La recategorización del Monotributo es semestral: en enero y julio el contribuyente revisa sus categorías.",
		"Si superás los parámetros de tu categoría del Monotributo debés recategorizarte en las categorías superiores.",
		"El Monotributo vigente agrupa impuesto integrado y aportes; las categorías dependen de los ingresos brutos.",
		"Exclusión del Monotributo: superar la categoría máxima K de ingresos brutos o los parámetros físicos.",
		"Categorías del Monotributo para servicios y para venta de bienes: diferencias en la cuota mensual.",
		"Pago de la cuota mensual del Monotributo con VEP, débito automático o tarjeta.",
		"Adhesión al Monotributo: elegir la categoría según los ingresos brutos de los últimos 12 meses.",
		"Monotributo: credencial de pago, constancia de inscripción y categorías vigentes en el período.",
		"Monotributo y categorías: cómo consultar la categoría actual en el portal de ARCA.",
		"Baja del Monotributo y pase al régimen general cuando se excluyen las categorías.",
	} {
		chunks = append(chunks, DocumentChunk{ChunkID: fmt.Sprintf("a_mono_%02d", i), FileName: fmt.Sprintf("ARCA_Monotributo_Guia_%d.pdf", i), PageNumber: 1, Content: text})
	}
	for i := range chunks {
		chunks[i].TenantID = GlobalTenant
	}
	return chunks
}

const categoriasQuestion = "¿Cuáles son las categorías del Monotributo vigentes de febrero a julio 2026?"

// expansionVariants are term sets the query expander may return for the same
// question in different calls (SPEC-CORE-70.R1 is not deterministic). Some
// reach page 1 only by joining file name words with content words
// ("ingresos brutos anuales monotributo"), which is what R7 adds.
//
// Known limit (not covered by R6/R7, re-ranking is out of scope): if page 1
// matches only through the file name and ties with more than topK other
// Monotributo pages (e.g. {"Monotributo categorías", "tabla de categorías",
// "cuota mensual"}), the tie-break decides and the table can be left out.
var expansionVariants = [][]string{
	{"categorías del Monotributo", "escala de ingresos brutos anuales", "impuesto integrado"},
	{"monto de la cuota por categoría", "ingresos brutos anuales monotributo"},
	{"régimen simplificado", "categorías A a K", "parámetros de categorización"},
	{"recategorización monotributo", "categorías monotributo 2026", "montos semestrales"},
	{"Monotributo", "categorías"},
}

func hasPage1(hits []SearchResult) bool {
	for _, h := range hits {
		if h.Chunk.ChunkID == "z_cat_p1" {
			return true
		}
	}
	return false
}

// SPEC-CORE-72.R8: with topK 10 (R6) and the file name words (R7), page 1 of
// the categories PDF (the table) is retrieved with every expansion variant.
func TestSPEC_CORE_72_R8_CategoriesTableIsAlwaysRetrieved(t *testing.T) {
	ctx := tenant.WithTenantID(context.Background(), "tenant_afip")
	idx := NewMemoryLexical()
	if err := idx.IndexChunks(ctx, arcaCorpus(categoriasPDF)); err != nil {
		t.Fatal(err)
	}
	const topK = 10 // router.DefaultRAGTopK
	for rep := 0; rep < 5; rep++ {
		for i, v := range expansionVariants {
			hits, err := idx.Search(ctx, append([]string{categoriasQuestion}, v...), topK)
			if err != nil {
				t.Fatal(err)
			}
			if !hasPage1(hits) {
				t.Errorf("repetition %d, variant %d %q: page 1 (table) missing from %d hits", rep, i, v, len(hits))
			}
		}
	}
}

// Control for R8: the same corpus as before SPEC-CORE-72 (topK 5 and a page
// that cannot match the title words) loses the table for some variants, which
// is the instability seen in production.
func TestSPEC_CORE_72_R8_ControlWithoutFixLosesTheTable(t *testing.T) {
	ctx := tenant.WithTenantID(context.Background(), "tenant_afip")
	idx := NewMemoryLexical()
	// A file name without title words emulates search_text = content only.
	if err := idx.IndexChunks(ctx, arcaCorpus("tabla.pdf")); err != nil {
		t.Fatal(err)
	}
	missed := 0
	for _, v := range expansionVariants {
		hits, err := idx.Search(ctx, append([]string{categoriasQuestion}, v...), 5)
		if err != nil {
			t.Fatal(err)
		}
		if !hasPage1(hits) {
			missed++
		}
	}
	if missed == 0 {
		t.Error("control corpus should reproduce the missing table; the R8 test would prove nothing")
	}
}
