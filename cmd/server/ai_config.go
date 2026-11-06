package main

import (
	"log"
	"regexp"
	"strconv"
	"strings"

	"github.com/martinllanos/only-ai-pods/internal/embed"
	"github.com/martinllanos/only-ai-pods/internal/llm"
	"github.com/martinllanos/only-ai-pods/internal/rag"
)

// aiStack is the LLM + RAG configuration chosen from the environment (SPEC-CORE-66.R7).
type aiStack struct {
	store    rag.Store
	llm      llm.Client // nil = prototype mode
	llmMode  string
	embedder string
}

var nonAlnum = regexp.MustCompile(`[^a-z0-9]+`)

// buildAIStack reads ANTHROPIC_API_KEY, ANTHROPIC_MODEL, OLLAMA_URL, EMBED_MODEL,
// EMBED_DIM, QDRANT_URL, QDRANT_API_KEY and QDRANT_COLLECTION.
func buildAIStack(getenv func(string) string) aiStack {
	var e embed.Embedder
	if url := getenv("OLLAMA_URL"); url != "" {
		model := getenv("EMBED_MODEL")
		if model == "" {
			model = "bge-m3"
		}
		dim := 1024
		if v, err := strconv.Atoi(getenv("EMBED_DIM")); err == nil && v > 0 {
			dim = v
		}
		e = embed.NewOllamaEmbedder(url, model, dim)
	} else {
		log.Println("WARNING: OLLAMA_URL not set; using non-semantic hash embeddings (development only)")
		e = embed.NewHashEmbedder(256)
	}

	var store rag.Store
	if url := getenv("QDRANT_URL"); url != "" {
		collection := getenv("QDRANT_COLLECTION")
		if collection == "" {
			// One collection per embedding model: vectors of different models
			// (or dimensions) must never be mixed.
			collection = "aipods_chunks_" + strings.Trim(nonAlnum.ReplaceAllString(strings.ToLower(e.Name()), "_"), "_")
		}
		store = rag.NewQdrantStore(url, getenv("QDRANT_API_KEY"), collection, e)
	} else {
		log.Println("WARNING: QDRANT_URL not set; using in-memory vector store")
		store = rag.NewMemoryStore(e, "", "aipods_vectors")
	}

	stack := aiStack{store: store, llmMode: "disabled", embedder: e.Name()}
	if key := getenv("ANTHROPIC_API_KEY"); key != "" {
		c := llm.NewAnthropicClient(key, getenv("ANTHROPIC_MODEL"))
		stack.llm = c
		stack.llmMode = "anthropic:" + c.Model()
	} else {
		log.Println("WARNING: ANTHROPIC_API_KEY not set; router and answers run in prototype mode")
	}
	return stack
}
