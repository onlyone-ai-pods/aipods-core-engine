package llm

import (
	"context"
	"errors"
	"reflect"
	"strings"
	"testing"
)

type recorder struct {
	out string
	err error
	got Request
}

func (r *recorder) Model() string { return "stub" }
func (r *recorder) Complete(_ context.Context, req Request) (string, error) {
	r.got = req
	return r.out, r.err
}

// SPEC-CORE-70.R1: an informal query comes back as formal ARCA terms.
func TestExpandQueryTranslatesInformalQuery(t *testing.T) {
	c := &recorder{out: "- Factura E\n- Exportación de servicios\n1. RG 5616\n2) \"Comprobantes clase E\"\n\nfactura e\n572\n"}
	terms, err := ExpandQuery(context.Background(), c, "cómo cobro laburo afuera")
	if err != nil {
		t.Fatal(err)
	}
	want := []string{"Factura E", "Exportación de servicios", "RG 5616", "Comprobantes clase E", "572"}
	if !reflect.DeepEqual(terms, want) {
		t.Errorf("terms = %q, want %q", terms, want)
	}
	if !strings.Contains(c.got.User, "<consulta>\ncómo cobro laburo afuera\n</consulta>") {
		t.Errorf("query not wrapped as data: %q", c.got.User)
	}
	if c.got.Effort != EffortLow || c.got.MaxTokens > 1000 {
		t.Errorf("expansion must be a quick call, got effort %q max %d", c.got.Effort, c.got.MaxTokens)
	}
}

func TestExpandQueryCapsTerms(t *testing.T) {
	var lines []string
	for i := 0; i < 30; i++ {
		lines = append(lines, "termino "+strings.Repeat("x", i+1))
	}
	terms, err := ExpandQuery(context.Background(), &recorder{out: strings.Join(lines, "\n")}, "q")
	if err != nil || len(terms) != MaxExpansionTerms {
		t.Errorf("got %d terms, %v", len(terms), err)
	}
}

func TestExpandQueryReturnsError(t *testing.T) {
	boom := errors.New("boom")
	terms, err := ExpandQuery(context.Background(), &recorder{err: boom}, "q")
	if !errors.Is(err, boom) || terms != nil {
		t.Errorf("got %q, %v", terms, err)
	}
}
