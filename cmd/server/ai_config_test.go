package main

import (
	"testing"
)

func envMap(m map[string]string) func(string) string {
	return func(k string) string { return m[k] }
}

// SPEC-CORE-70.R5: with no embedding settings the core starts without Ollama
// and without a vector store.
func TestBuildAIStackPrototypeMode(t *testing.T) {
	s := buildAIStack(envMap(nil))
	if s.llm != nil || s.llmMode != "disabled" {
		t.Errorf("expected LLM disabled, got %s", s.llmMode)
	}
	if s.store != nil || s.vectorStoreName() != "disabled" || s.embedder != "disabled" {
		t.Errorf("expected no vector store, got %s / %s", s.vectorStoreName(), s.embedder)
	}
}

func TestBuildAIStackWithoutOllama(t *testing.T) {
	s := buildAIStack(envMap(map[string]string{
		"ANTHROPIC_API_KEY": "test-key",
		"QDRANT_URL":        "http://qdrant:6333",
		"EMBED_DIM":         "1024",
	}))
	if s.llm == nil || s.store != nil {
		t.Errorf("Qdrant without an embedder must stay disabled, got %s", s.vectorStoreName())
	}
}

func TestBuildAIStackRealMode(t *testing.T) {
	s := buildAIStack(envMap(map[string]string{
		"ANTHROPIC_API_KEY": "test-key",
		"OLLAMA_URL":        "http://ollama:11434",
		"QDRANT_URL":        "http://qdrant:6333",
	}))
	if s.llm == nil || s.llmMode != "anthropic:claude-opus-5-5" {
		t.Errorf("unexpected llm mode %s", s.llmMode)
	}
	if s.vectorStoreName() != "qdrant+ollama:bge-m3" {
		t.Errorf("unexpected store %s", s.vectorStoreName())
	}
}

// SPEC-CORE-70.R2: the SaaS embedders are picked from the environment.
func TestBuildEmbedderProviders(t *testing.T) {
	cases := []struct {
		env  map[string]string
		name string
		dim  int
	}{
		{map[string]string{"VOYAGE_API_KEY": "k"}, "voyage:voyage-3.5-lite", 1024},
		{map[string]string{"VOYAGE_API_KEY": "k", "EMBED_MODEL": "voyage-3-lite", "EMBED_DIM": "512"}, "voyage:voyage-3-lite", 512},
		{map[string]string{"OPENAI_API_KEY": "k"}, "openai:text-embedding-3-small", 1536},
		{map[string]string{"OPENAI_API_KEY": "k", "EMBED_DIM": "1024"}, "openai:text-embedding-3-small", 1024},
		{map[string]string{"EMBED_PROVIDER": "voyage", "OLLAMA_URL": "http://ollama:11434", "VOYAGE_API_KEY": "k"}, "voyage:voyage-3.5-lite", 1024},
		{map[string]string{"EMBED_PROVIDER": "hash"}, "hash", 256},
	}
	for _, c := range cases {
		e := buildEmbedder(envMap(c.env))
		if e == nil || e.Name() != c.name || e.Dimension() != c.dim {
			t.Errorf("%v: got %v", c.env, e)
		}
	}
	for _, env := range []map[string]string{{"EMBED_PROVIDER": "none", "OLLAMA_URL": "http://x"}, {"EMBED_PROVIDER": "ollama"}, {"EMBED_PROVIDER": "bogus"}} {
		if e := buildEmbedder(envMap(env)); e != nil {
			t.Errorf("%v: expected no embedder, got %s", env, e.Name())
		}
	}
}

func TestQdrantCollectionPerEmbedder(t *testing.T) {
	s := buildAIStack(envMap(map[string]string{"VOYAGE_API_KEY": "k", "QDRANT_URL": "http://qdrant:6333"}))
	if s.vectorStoreName() != "qdrant+voyage:voyage-3.5-lite" {
		t.Errorf("got %s", s.vectorStoreName())
	}
}
