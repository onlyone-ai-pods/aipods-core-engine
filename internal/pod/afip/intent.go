package afip

import (
	"strings"
	"unicode"

	"github.com/martinllanos/only-ai-pods/internal/rag"
)

// Intent of a query to the AFIP pod (SPEC-CORE-72.R1).
type Intent string

const (
	// IntentInformational is a question about rules or procedures: it is
	// answered from the documents, without actions (SPEC-CORE-72.R2).
	IntentInformational Intent = "informational"
	// IntentOperational asks to act on, or read, the client's own data in
	// ARCA: it produces a dry-run that needs human approval (SPEC-CORE-72.R3).
	IntentOperational Intent = "operational"
)

// object is the operational object a query talks about. The order is the
// priority when a query names more than one (same order as the old branches).
type object int

const (
	objNone object = iota
	objRetenciones
	objPuntoDeVenta
	objComprobantes
	objEstadoDeCuenta
	objCSR
)

// objectPhrases are the operational objects as sequences of normalized words.
var objectPhrases = []struct {
	obj    object
	phrase []string
}{
	{objRetenciones, []string{"retencion"}},
	{objRetenciones, []string{"retenciones"}},
	{objRetenciones, []string{"percepcion"}},
	{objRetenciones, []string{"percepciones"}},
	{objRetenciones, []string{"sicore"}},
	{objRetenciones, []string{"sire"}},
	{objPuntoDeVenta, []string{"punto", "de", "venta"}},
	{objPuntoDeVenta, []string{"puntos", "de", "venta"}},
	{objPuntoDeVenta, []string{"pv"}},
	{objComprobantes, []string{"comprobante"}},
	{objComprobantes, []string{"comprobantes"}},
	{objComprobantes, []string{"factura"}},
	{objComprobantes, []string{"facturas"}},
	{objEstadoDeCuenta, []string{"estado", "de", "cuenta"}},
	{objCSR, []string{"csr"}},
	{objCSR, []string{"certificado"}},
	{objCSR, []string{"certificados"}},
}

// verbForms are the action verbs in infinitive, imperative (voseo, tú and
// usted, which lose the accent once normalized) and first person. Forms that
// are also common non-verbs ("bajo", "creo", "cree") are left out.
var verbForms = []string{
	"consultar", "consulta", "consulte", "consulto",
	"ver", "vea", "veo",
	"mostrar", "mostra", "muestra", "muestre", "muestro",
	"revisar", "revisa", "revise", "reviso",
	"descargar", "descarga", "descargue", "descargo",
	"bajar", "baja", "baje",
	"traer", "trae", "traiga", "traigo",
	"exportar", "exporta", "exporte", "exporto",
	"crear", "crea",
	"agregar", "agrega", "agregue", "agrego",
	"habilitar", "habilita", "habilite", "habilito",
	"generar", "genera", "genere", "genero",
	"emitir", "emiti", "emite", "emita", "emito",
}

// darForms precede "de alta" / "de baja" ("dar de alta", "doy de alta", "dé de baja").
var darForms = []string{"dar", "da", "de", "doy", "demos", "den", "dame", "dale", "darle", "darme", "darlo", "darla"}

// clitics turn "mostrá" into "mostrame", "descargá" into "descargalos", etc.
var clitics = []string{"", "me", "nos", "lo", "la", "los", "las", "le", "les"}

var actionVerbs = func() map[string]bool {
	m := make(map[string]bool)
	for _, v := range verbForms {
		for _, c := range clitics {
			m[v+c] = true
		}
	}
	return m
}()

var possessives = map[string]bool{"mi": true, "mis": true, "nuestro": true, "nuestra": true, "nuestros": true, "nuestras": true}

// possessiveWindow is how many words may separate a possessive from its
// object: "mis comprobantes" (1), "mi archivo CSR" (2).
const possessiveWindow = 3

// accentedMarkers are question words that are only unambiguous with their
// accent ("cómo" vs "como", "qué es" vs "que es"). Without the accent they
// count only at the start of the query, where users often skip accents.
var accentedMarkers = [][]string{
	{"cómo"}, {"qué", "es"}, {"qué", "son"}, {"cuál"}, {"cuáles"},
	{"cuándo"}, {"dónde"}, {"por", "qué"}, {"para", "qué"},
}

// plainMarkers are tutorial phrases matched anywhere on normalized words.
var plainMarkers = [][]string{
	{"requisito"}, {"requisitos"}, {"pasos", "para"}, {"se", "puede"},
	{"hace", "falta"}, {"necesito", "saber"},
}

// classifyIntent decides whether a query is operational or informational
// (SPEC-CORE-72.R1). It compares whole normalized words, never substrings:
// "pv" does not match inside "previo". It also returns the operational object
// that selects the action.
func classifyIntent(query string) (Intent, object) {
	raw := splitWords(strings.ToLower(query))
	norm := make([]string, len(raw))
	for i, w := range raw {
		norm[i] = rag.Normalize(w)
	}

	objects := findObjects(norm)
	first := objNone
	for _, o := range []object{objRetenciones, objPuntoDeVenta, objComprobantes, objEstadoDeCuenta, objCSR} {
		if _, ok := objects[o]; ok {
			first = o
			break
		}
	}

	// Own data: "estado de cuenta" is always the client's, and so is any
	// operational object with a client possessive. This wins over questions:
	// "¿cuál es mi estado de cuenta?" is operational.
	if _, ok := objects[objEstadoDeCuenta]; ok {
		return IntentOperational, objEstadoDeCuenta
	}
	// The action follows the object priority, as the old branches did:
	// "¿qué retenciones tengo en mis comprobantes?" reads retenciones.
	if possessiveObject(norm, objects) != objNone {
		return IntentOperational, first
	}

	// Questions and tutorials are informational even with verb + object:
	// "¿Cómo doy de alta un punto de venta?".
	if isQuestion(query, raw, norm) {
		return IntentInformational, objNone
	}

	if first != objNone && hasActionVerb(norm) {
		return IntentOperational, first
	}
	return IntentInformational, objNone
}

// splitWords splits on anything that is not a letter or a digit.
func splitWords(s string) []string {
	return strings.FieldsFunc(s, func(r rune) bool {
		return !unicode.IsLetter(r) && !unicode.IsDigit(r)
	})
}

// findObjects returns each operational object found with the word indexes where it starts.
func findObjects(words []string) map[object][]int {
	out := make(map[object][]int)
	for _, op := range objectPhrases {
		for _, i := range phraseAt(words, op.phrase) {
			out[op.obj] = append(out[op.obj], i)
		}
	}
	return out
}

// phraseAt returns every index where phrase starts in words.
func phraseAt(words, phrase []string) []int {
	var at []int
	for i := 0; i+len(phrase) <= len(words); i++ {
		match := true
		for j, p := range phrase {
			if words[i+j] != p {
				match = false
				break
			}
		}
		if match {
			at = append(at, i)
		}
	}
	return at
}

// possessiveObject returns the first object (by priority) preceded by a
// client possessive within possessiveWindow words.
func possessiveObject(words []string, objects map[object][]int) object {
	for _, o := range []object{objRetenciones, objPuntoDeVenta, objComprobantes, objEstadoDeCuenta, objCSR} {
		for _, start := range objects[o] {
			for i := start - 1; i >= 0 && start-i <= possessiveWindow; i-- {
				if possessives[words[i]] {
					return o
				}
			}
		}
	}
	return objNone
}

func isQuestion(query string, raw, norm []string) bool {
	if strings.ContainsAny(query, "?¿") {
		return true
	}
	for _, m := range accentedMarkers {
		if len(phraseAt(raw, m)) > 0 {
			return true
		}
		folded := make([]string, len(m))
		for i, w := range m {
			folded[i] = rag.Normalize(w)
		}
		if at := phraseAt(norm, folded); len(at) > 0 && at[0] == 0 {
			return true
		}
	}
	for _, m := range plainMarkers {
		if len(phraseAt(norm, m)) > 0 {
			return true
		}
	}
	return false
}

func hasActionVerb(words []string) bool {
	for i, w := range words {
		if actionVerbs[w] {
			return true
		}
		// "dar de alta", "doy de baja"
		if i+2 < len(words) && words[i+1] == "de" && (words[i+2] == "alta" || words[i+2] == "baja") {
			for _, d := range darForms {
				if w == d {
					return true
				}
			}
		}
	}
	return false
}
