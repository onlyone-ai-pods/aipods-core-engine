package afip

import (
	"context"
	"strings"
	"testing"
)

func TestAFIPPodCSRQuery(t *testing.T) {
	pod := NewAFIPPod()
	ctx := context.Background()
	tenantID := "TENANT_DEMO_001"

	res, err := pod.ProcessQuery(ctx, tenantID, "Cómo genero mi archivo CSR para AFIP?", true)
	if err != nil {
		t.Fatalf("Expected no error, got: %v", err)
	}

	if res.PodID != "POD_AFIP_FISCAL" {
		t.Errorf("Expected PodID POD_AFIP_FISCAL, got: %s", res.PodID)
	}

	// Pods never invent citations: real ones come only from RAG hits (SPEC-CORE-66).
	if len(res.Citations) != 0 {
		t.Errorf("Expected no hardcoded citations, got %v", res.Citations)
	}

	if res.DryRunResult == nil || !res.DryRunResult.IsDryRun {
		t.Errorf("Expected DryRunResult to be active (is_dry_run = true)")
	}

	if res.DryRunResult.GeneratedCommand != "openssl req -new -key privada.key -out pedido.csr" {
		t.Errorf("Unexpected generated command: %s", res.DryRunResult.GeneratedCommand)
	}
}

func TestAFIPPodComprobantesQuery(t *testing.T) {
	pod := NewAFIPPod()
	ctx := context.Background()
	tenantID := "TENANT_DEMO_001"

	resDry, err := pod.ProcessQuery(ctx, tenantID, "Quiero consultar mis comprobantes emitidos en AFIP", true)
	if err != nil {
		t.Fatalf("Expected no error on dry-run, got: %v", err)
	}

	if resDry.DryRunResult == nil || !resDry.DryRunResult.IsDryRun {
		t.Errorf("Expected DryRunResult to be active on dry-run")
	}

	if resDry.DryRunResult.ActionName != "descargar_comprobantes_arca" {
		t.Errorf("Expected ActionName descargar_comprobantes_arca, got: %s", resDry.DryRunResult.ActionName)
	}

	if !strings.Contains(resDry.Answer, "Dry-Run Simulation") {
		t.Errorf("Expected dry-run answer header")
	}
}

func TestAFIPPodPuntosDeVentaQuery(t *testing.T) {
	pod := NewAFIPPod()
	ctx := context.Background()
	tenantID := "TENANT_DEMO_001"

	resDry, err := pod.ProcessQuery(ctx, tenantID, "Consulta mis puntos de venta en ARCA", true)
	if err != nil {
		t.Fatalf("Expected no error on dry-run, got: %v", err)
	}

	if resDry.DryRunResult == nil || !resDry.DryRunResult.IsDryRun {
		t.Errorf("Expected DryRunResult to be active on dry-run")
	}

	if resDry.DryRunResult.ActionName != "gestionar_puntos_de_venta_arca" {
		t.Errorf("Expected ActionName gestionar_puntos_de_venta_arca, got: %s", resDry.DryRunResult.ActionName)
	}
}

func TestAFIPPodRetencionesQuery(t *testing.T) {
	pod := NewAFIPPod()
	ctx := context.Background()
	tenantID := "TENANT_DEMO_001"

	resDry, err := pod.ProcessQuery(ctx, tenantID, "Descargá mis retenciones sufridas de IVA y Ganancias en ARCA", true)
	if err != nil {
		t.Fatalf("Expected no error on dry-run, got: %v", err)
	}

	if resDry.DryRunResult == nil || !resDry.DryRunResult.IsDryRun {
		t.Errorf("Expected DryRunResult to be active on dry-run")
	}

	if resDry.DryRunResult.ActionName != "descargar_retenciones_arca" {
		t.Errorf("Expected ActionName descargar_retenciones_arca, got: %s", resDry.DryRunResult.ActionName)
	}
}

func TestRemoveStopWordsKeepsWholeWords(t *testing.T) {
	got := removeStopWords("¿cuáles son los tipos de punto de venta para factura electrónica?")
	if got != "factura electrónica" {
		t.Errorf("got %q", got)
	}
	if got := removeStopWords("ver punto de venta 00007"); got != "00007" {
		t.Errorf("got %q", got)
	}
}

// SPEC-CORE-72.R1: case table of the spec plus edge cases of the rules.
func TestSPEC_CORE_72_R1_ClassifyIntent(t *testing.T) {
	cases := []struct {
		query string
		want  Intent
	}{
		// Spec table: informational.
		{"¿Cuáles son las categorías del Monotributo vigentes?", IntentInformational},
		{"¿Qué es la clave fiscal?", IntentInformational},
		{"¿Cómo doy de alta un punto de venta?", IntentInformational},
		{"Pasos para generar el CSR", IntentInformational},
		{"¿Qué requisitos hay para crear un punto de venta?", IntentInformational},
		// Spec table: operational.
		{"¿Cuál es mi estado de cuenta?", IntentOperational},
		{"Consultá mi estado de cuenta del monotributo", IntentOperational},
		{"Descargá mis comprobantes recibidos de octubre", IntentOperational},
		{"Creá un punto de venta RECE", IntentOperational},
		{"Generá el CSR para el certificado", IntentOperational},
		// Own data inside a question stays operational (R1 exception).
		{"¿Qué retenciones tengo en mis comprobantes?", IntentOperational},
		{"Cómo genero mi archivo CSR para AFIP?", IntentOperational},
		// Questions without accents or question marks.
		{"como doy de alta un punto de venta", IntentInformational},
		{"cuales son las categorias del monotributo", IntentInformational},
		{"Se puede emitir factura E desde el monotributo", IntentInformational},
		{"Necesito saber qué comprobantes emitir", IntentInformational},
		{"Requisitos para habilitar un punto de venta", IntentInformational},
		// Whole words, not substrings: "pv" is not inside "previo".
		{"Revisá el paso previo a la facturación", IntentInformational},
		{"Revisá el PV 00007", IntentOperational},
		// Verb without object, object without verb: informational.
		{"Consultá las categorías del monotributo", IntentInformational},
		{"Factura E para exportación de servicios", IntentInformational},
		{"Monotributo categoría A", IntentInformational},
		// Verb + object (infinitive, clitic, dar de alta).
		{"Quiero consultar los comprobantes emitidos", IntentOperational},
		{"Mostrame las percepciones de IVA", IntentOperational},
		{"Dar de alta un punto de venta para Odoo", IntentOperational},
		{"Consulta mis puntos de venta en ARCA", IntentOperational},
		{"Descargá mis retenciones sufridas de IVA y Ganancias en ARCA", IntentOperational},
		{"descargar mis retenciones sicore", IntentOperational},
		{"explicame la normativa general de ganancias", IntentInformational},
	}
	for _, tc := range cases {
		t.Run(tc.query, func(t *testing.T) {
			if got, _ := classifyIntent(tc.query); got != tc.want {
				t.Errorf("classifyIntent(%q) = %s, want %s", tc.query, got, tc.want)
			}
		})
	}
}

// SPEC-CORE-72.R2: informational answers carry no action, no approval token
// and no client data.
func TestSPEC_CORE_72_R2_InformationalAnswerHasNoActionNorTenant(t *testing.T) {
	p := NewAFIPPod()
	const tenantID = "TENANT_DEMO_001"
	for _, q := range []string{
		"¿Cuáles son las categorías del Monotributo vigentes?",
		"¿Qué es la clave fiscal?",
		"¿Cómo doy de alta un punto de venta?",
		"Pasos para generar el CSR",
		"explicame la normativa general de ganancias",
	} {
		for _, dryRun := range []bool{true, false} {
			res, err := p.ProcessQuery(context.Background(), tenantID, q, dryRun)
			if err != nil {
				t.Fatalf("%q: %v", q, err)
			}
			if res.DryRunResult != nil {
				t.Errorf("%q (dryRun=%t): informational query must not produce an action: %+v", q, dryRun, res.DryRunResult)
			}
			if res.Intent != string(IntentInformational) {
				t.Errorf("%q: intent %q", q, res.Intent)
			}
			low := strings.ToLower(res.Answer)
			if strings.Contains(low, strings.ToLower(tenantID)) || strings.Contains(res.Answer, "20262534538") {
				t.Errorf("%q: informational answer leaks client data: %q", q, res.Answer)
			}
		}
	}
}

// SPEC-CORE-72.R3: operational queries keep the dry-run with human approval.
func TestSPEC_CORE_72_R3_OperationalKeepsDryRunApproval(t *testing.T) {
	p := NewAFIPPod()
	cases := map[string]string{
		"¿Cuál es mi estado de cuenta?":                  "consultar_monotributo_estado_cuenta",
		"Consultá mi estado de cuenta del monotributo":   "consultar_monotributo_estado_cuenta",
		"Descargá mis comprobantes recibidos de octubre": "descargar_comprobantes_arca",
		"Creá un punto de venta RECE":                    "gestionar_puntos_de_venta_arca",
		"Generá el CSR para el certificado":              "generar_csr_afip",
		"Mostrame las percepciones de IVA":               "descargar_retenciones_arca",
	}
	for q, action := range cases {
		res, err := p.ProcessQuery(context.Background(), "TENANT_DEMO_001", q, true)
		if err != nil {
			t.Fatalf("%q: %v", q, err)
		}
		dr := res.DryRunResult
		if dr == nil || !dr.RequiresHumanApproval || dr.ApprovalToken == "" || dr.ActionName != action {
			t.Errorf("%q: want action %s with approval, got %+v", q, action, dr)
		}
		if res.Intent != string(IntentOperational) {
			t.Errorf("%q: intent %q", q, res.Intent)
		}
	}
}
