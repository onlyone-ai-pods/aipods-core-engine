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

// aiStack is the LLM + RAG configuration chosen from the environment
// (SPEC-CORE-66.R7, SPEC-CORE-70).
type aiStack struct {
	store    rag.Store  // nil = no embedding model, lexical search only (SPEC-CORE-70.R5)
	llm      llm.Client // nil = prototype mode
	llmMode  string
	embedder string
}

var nonAlnum = regexp.MustCompile(`[^a-z0-9]+`)

// buildEmbedder picks the embedding model. EMBED_PROVIDER is ollama, voyage,
// openai, hash or none; when unset it is inferred from OLLAMA_URL,
// VOYAGE_API_KEY or OPENAI_API_KEY, in that order. With none of them the core
// runs without embeddings: Ollama is optional (SPEC-CORE-70.R5).
func buildEmbedder(getenv func(string) string) embed.Embedder {
	provider := strings.ToLower(strings.TrimSpace(getenv("EMBED_PROVIDER")))
	if provider == "" {
		switch {
		case getenv("OLLAMA_URL") != "":
			provider = "ollama"
		case getenv("VOYAGE_API_KEY") != "":
			provider = "voyage"
		case getenv("OPENAI_API_KEY") != "":
			provider = "openai"
		default:
			provider = "none"
		}
	}
	model := getenv("EMBED_MODEL")
	dim, _ := strconv.Atoi(getenv("EMBED_DIM"))

	switch provider {
	case "ollama":
		url := getenv("OLLAMA_URL")
		if url == "" {
			log.Println("WARNING: EMBED_PROVIDER=ollama but OLLAMA_URL is not set; embeddings disabled")
			return nil
		}
		if model == "" {
			model = "bge-m3"
		}
		if dim <= 0 {
			dim = 1024
		}
		return embed.NewOllamaEmbedder(url, model, dim)
	case "voyage":
		return embed.NewVoyageEmbedder(getenv("VOYAGE_API_KEY"), model, dim, getenv("VOYAGE_BASE_URL"))
	case "openai":
		return embed.NewOpenAIEmbedder(getenv("OPENAI_API_KEY"), model, dim, getenv("OPENAI_BASE_URL"))
	case "hash":
		log.Println("WARNING: EMBED_PROVIDER=hash uses non-semantic embeddings (development only)")
		return embed.NewHashEmbedder(256)
	case "none":
		log.Println("RAG: no embedding model configured; documents are searched with query expansion + lexical search")
		return nil
	default:
		log.Printf("WARNING: unknown EMBED_PROVIDER %q; embeddings disabled", provider)
		return nil
	}
}

// buildAIStack reads ANTHROPIC_API_KEY, ANTHROPIC_MODEL, the embedding
// settings (see buildEmbedder), QDRANT_URL, QDRANT_API_KEY and QDRANT_COLLECTION.
func buildAIStack(getenv func(string) string) aiStack {
	stack := aiStack{llmMode: "disabled", embedder: "disabled"}

	if e := buildEmbedder(getenv); e != nil {
		stack.embedder = e.Name()
		if url := getenv("QDRANT_URL"); url != "" {
			collection := getenv("QDRANT_COLLECTION")
			if collection == "" {
				// One collection per embedding model: vectors of different models
				// (or dimensions) must never be mixed.
				collection = "aipods_chunks_" + strings.Trim(nonAlnum.ReplaceAllString(strings.ToLower(e.Name()), "_"), "_")
			}
			stack.store = rag.NewQdrantStore(url, getenv("QDRANT_API_KEY"), collection, e)
		} else {
			log.Println("WARNING: QDRANT_URL not set; using in-memory vector store")
			stack.store = rag.NewMemoryStore(e, "", "aipods_vectors")
		}
	}

	if key := getenv("ANTHROPIC_API_KEY"); key != "" {
		c := llm.NewAnthropicClient(key, getenv("ANTHROPIC_MODEL"))
		stack.llm = c
		stack.llmMode = "anthropic:" + c.Model()
	} else {
		log.Println("WARNING: ANTHROPIC_API_KEY not set; router and answers run in prototype mode")
	}
	return stack
}

// vectorStoreName is shown in /healthz.
func (s aiStack) vectorStoreName() string {
	if s.store == nil {
		return "disabled"
	}
	return s.store.Name()
}
