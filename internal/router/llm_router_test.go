package router

import (
	"context"
	"errors"
	"strings"
	"testing"

	"github.com/martinllanos/only-ai-pods/internal/llm"
	"github.com/martinllanos/only-ai-pods/internal/rag"
	"github.com/martinllanos/only-ai-pods/internal/tenant"
)

// fakeLLM answers the classifier with classifyAnswer and the drafter with draftAnswer.
type fakeLLM struct {
	classifyAnswer string
	draftAnswer    string
	classifyErr    error
	draftErr       error
	lastDraft      string
}

func (f *fakeLLM) Model() string { return "fake-model" }

func (f *fakeLLM) Complete(_ context.Context, req llm.Request) (string, error) {
	if req.Effort == llm.EffortLow {
		return f.classifyAnswer, f.classifyErr
	}
	f.lastDraft = req.User
	return f.draftAnswer, f.draftErr
}

type fakeRetriever struct{ gotTenant string }

func (f *fakeRetriever) Search(ctx context.Context, _ string, _ int) ([]rag.SearchResult, error) {
	f.gotTenant, _ = tenant.FromContext(ctx)
	return []rag.SearchResult{{
		Chunk:      rag.DocumentChunk{Content: "El vencimiento de SICORE es el día 10."},
		CitationID: "Calendario_ARCA.pdf (Pagina 2)",
	}}, nil
}

func TestLLMRoutingPicksClassifiedPod(t *testing.T) {
	r := NewDynamicSmartRouter()
	r.SetLLM(&fakeLLM{classifyAnswer: "POD_SCM_LOGISTICS", draftAnswer: "respuesta"})
	// No SCM keyword in the query: only the LLM can route it there.
	res, err := r.RouteAndExecute(context.Background(), "tenant_acme", "necesito reponer stock del depósito", true)
	if err != nil {
		t.Fatal(err)
	}
	if res.PodID != "POD_SCM_LOGISTICS" {
		t.Errorf("expected POD_SCM_LOGISTICS, got %s", res.PodID)
	}
}

func TestLLMRoutingFallsBackOnUnknownID(t *testing.T) {
	for name, f := range map[string]*fakeLLM{
		"unknown id": {classifyAnswer: "POD_INVENTADO", draftAnswer: "x"},
		"error":      {classifyErr: errors.New("429"), draftAnswer: "x"},
	} {
		r := NewDynamicSmartRouter()
		r.SetLLM(f)
		res, err := r.RouteAndExecute(context.Background(), "tenant_acme", "consultar pedidos en SAP S/4HANA", true)
		if err != nil {
			t.Fatal(err)
		}
		if res.PodID != "POD_SAP_ENTERPRISE" {
			t.Errorf("%s: expected keyword fallback to POD_SAP_ENTERPRISE, got %s", name, res.PodID)
		}
	}
}

func TestDraftAnswerUsesRAGAndKeepsDryRun(t *testing.T) {
	f := &fakeLLM{classifyAnswer: "POD_AFIP_FISCAL", draftAnswer: "Vence el día 10 [Calendario_ARCA.pdf (Pagina 2)]."}
	ret := &fakeRetriever{}
	r := NewDynamicSmartRouter()
	r.SetLLM(f)
	r.SetRetriever(ret)

	res, err := r.RouteAndExecute(context.Background(), "tenant_acme", "descargar mis retenciones sicore", true)
	if err != nil {
		t.Fatal(err)
	}
	if res.Answer != f.draftAnswer || res.Model != "fake-model" {
		t.Errorf("answer not drafted by LLM: %q / %q", res.Answer, res.Model)
	}
	if ret.gotTenant != "tenant_acme" {
		t.Errorf("RAG search must run with the request tenant, got %q", ret.gotTenant)
	}
	if len(res.Citations) == 0 || res.Citations[0] != "Calendario_ARCA.pdf (Pagina 2)" {
		t.Errorf("missing RAG citation: %v", res.Citations)
	}
	if res.DryRunResult == nil || !res.DryRunResult.RequiresHumanApproval {
		t.Error("dry-run and human approval must be kept from the pod")
	}
	if !strings.Contains(f.lastDraft, "<documento") || !strings.Contains(f.lastDraft, "Requiere aprobación humana: true") {
		t.Error("draft prompt should include documents and the dry-run state")
	}
}

func TestDraftAnswerFailureReturnsPodResult(t *testing.T) {
	r := NewDynamicSmartRouter()
	r.SetLLM(&fakeLLM{classifyAnswer: "POD_SAP_ENTERPRISE", draftErr: errors.New("timeout")})
	res, err := r.RouteAndExecute(context.Background(), "tenant_acme", "pedidos SAP", true)
	if err != nil {
		t.Fatal(err)
	}
	if res.Answer == "" || res.Model != "" {
		t.Errorf("expected the pod's own answer when the LLM fails, got %q (model %q)", res.Answer, res.Model)
	}
}
