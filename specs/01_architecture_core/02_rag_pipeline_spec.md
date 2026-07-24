# 📜 SPEC: RAG Data Pipeline & Knowledge Retrieval
**ID:** SPEC-CORE-02  
**Épica Relacionada:** Épicas 6, 7 & 10 (HU 6.1, HU 7.1, HU 10.1)  
**Estado:** DRAFT / READY FOR IMPL  

---

## 1. Alcance
Especificación técnica formal para el procesamiento de documentos, vectorización, búsqueda semántica top-K, citación de fuentes e invalidación/re-indexación de vectores obsoletos.

---

## 2. Parámetros de Ingesta & Chunking

| Parámetro | Valor Especificado | Tolerancia / Regla |
| :--- | :--- | :--- |
| **Tamaño de Chunk (Tokens)** | 512 tokens | Min: 256, Max: 1024 |
| **Overlap (Tokens)** | 64 tokens | 12.5% del chunk size |
| **Embedding Model** | `text-embedding-3-small` | 1536 dimensiones |
| **Top-K Retrieval** | 4 fragmentos | Ajustable según context window |
| **Threshold Mínimo de Similitud** | 0.72 (Cosine Distance) | Chunks con score < 0.72 son ignorados |

---

## 3. Escenarios de Comportamiento (BDD)

### Escenario 1: Generación de respuesta con Citas Explícitas
```gherkin
Given un documento "Resolución_AFIP_4577.pdf" subido por el admin para el dominio "AFIP"
And fragmentos vectorizados asociados en la BD Vectorial
When un cliente pregunta "¿Cuál es el límite de facturación electrónica según la norma 4577?"
Then el motor RAG debe recuperar los fragmentos relevantes de "Resolución_AFIP_4577.pdf"
And la respuesta generada por el LLM DEBE contener la cita explícita "[Fuente: Resolución_AFIP_4577.pdf, Pág. X]"
And NO se debe generar información fuera del contexto provisto (Anti-alucinación)
```

### Escenario 2: Reemplazo e Invalidación de Vectores (Data CI/CD)
```gherkin
Given un documento activo "Manual_AFIP_v1.pdf" con doc_id "doc_123" y estado "ACTIVE"
When el socio Senior sube "Manual_AFIP_v2.pdf" indicando replaces_doc_id="doc_123"
Then el sistema debe actualizar el estado de "doc_123" en PostgreSQL a "OBSOLETE"
And el filtro status=="ACTIVE" en la BD Vectorial debe excluir automáticamente los vectores viejos
And los nuevos vectores de "Manual_AFIP_v2.pdf" deben quedar inmediatamente disponibles para búsquedas RAG
```

---

## 4. Contrato de Respuesta RAG

```json
{
  "answer": "string",
  "citations": [
    {
      "doc_id": "uuid",
      "doc_name": "string",
      "page_number": "integer",
      "score": "float",
      "snippet": "string"
    }
  ],
  "context_used_count": "integer"
}
```
