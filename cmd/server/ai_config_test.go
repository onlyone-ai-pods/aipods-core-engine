package main

import (
	"strings"
	"testing"
)

func envMap(m map[string]string) func(string) string {
	return func(k string) string { return m[k] }
}

func TestBuildAIStackPrototypeMode(t *testing.T) {
	s := buildAIStack(envMap(nil))
	if s.llm != nil || s.llmMode != "disabled" {
		t.Errorf("expected LLM disabled, got %s", s.llmMode)
	}
	if !strings.HasPrefix(s.store.Name(), "memory+hash") {
		t.Errorf("expected memory store with hash embedder, got %s", s.store.Name())
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
	if s.store.Name() != "qdrant+ollama:bge-m3" {
		t.Errorf("unexpected store %s", s.store.Name())
	}
}
