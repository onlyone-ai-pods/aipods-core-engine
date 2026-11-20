package main

import (
	"fmt"
	"log"
	"regexp"
	"strconv"
	"strings"
	"time"

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
	// llmTimeout bounds each LLM call (LLM_TIMEOUT, SPEC-CORE-73.R3); zero when there is no LLM.
	llmTimeout time.Duration
	// models is the model per task, the fallback chain and the allowed list
	// (SPEC-CORE-74).
	models llmSettings
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
// settings (see buildEmbedder), QDRANT_URL, QDRANT_API_KEY and QDRANT_COLLECTION,
// plus the model per task of SPEC-CORE-74 (see loadLLMSettings). It fails
// when a configured model is not allowed.
func buildAIStack(getenv func(string) string) (aiStack, error) {
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

	baseModel := strings.TrimSpace(getenv("ANTHROPIC_MODEL"))
	if baseModel == "" {
		baseModel = llm.DefaultModel
	}
	llmTimeout := durationEnv(getenv, "LLM_TIMEOUT", llm.DefaultTimeout)
	models, err := loadLLMSettings(getenv, baseModel, llmTimeout)
	if err != nil {
		return aiStack{}, fmt.Errorf("llm: %w", err)
	}
	stack.models = models

	if key := getenv("ANTHROPIC_API_KEY"); key != "" {
		c := llm.NewAnthropicClient(key, baseModel)
		c.SetTimeout(llmTimeout)
		stack.llm = c
		stack.llmTimeout = c.Timeout()
		// llm keeps its format: the model that drafts the answers (R16).
		stack.llmMode = "anthropic:" + models.defaults.Answer
	} else {
		log.Println("WARNING: ANTHROPIC_API_KEY not set; router and answers run in prototype mode")
	}
	return stack, nil
}

// llmRuntime wires the task clients to the metrics (SPEC-CORE-74.R15).
func (s aiStack) llmRuntime(onRequest func(task, model, outcome string), onFallback func(task, from, to string)) *llm.Runtime {
	return &llm.Runtime{
		Defaults:       s.models.defaults,
		Budget:         s.models.budget,
		AttemptTimeout: s.models.attemptTimeout,
		OnRequest:      func(t llm.Task, model, outcome string) { onRequest(string(t), model, outcome) },
		OnFallback:     func(t llm.Task, from, to string) { onFallback(string(t), from, to) },
	}
}

// vectorStoreName is shown in /healthz.
func (s aiStack) vectorStoreName() string {
	if s.store == nil {
		return "disabled"
	}
	return s.store.Name()
}
