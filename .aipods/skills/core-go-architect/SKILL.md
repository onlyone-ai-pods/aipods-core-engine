---
name: core-go-architect
description: Guía experta de arquitectura en Go 1.22+ para el motor backend de AI Pods Enterprise.
---

# 🚀 Skill: Core Go Architect

Esta habilidad instruye a asistentes de IA para desarrollar el backend del proyecto `aipods-core-engine` en **Golang 1.22+**:

## 1. Trazabilidad Estricta & Criterios de Aceptación SDD (Pre-Code Gate)
Antes de escribir cualquier código Go, consultar la especificación ejecutable en `specs/`. 

### Criterios de Aceptación Obligatorios para Ingesta RAG y Archivos:
- **Sanitización Pre-Ingesta Invariable:** TODO helper de ingesta de documentos (`pdf_ingest.go`, `md_ingest.go`, etc.) DEBE invocar obligatoriamente `FileSanitizer.ValidatePDFMagicBytes()` y `FileSanitizer.SanitizeTextContent()` ANTES de realizar cualquier fragmentación (*chunking*) o almacenamiento vectorial.
- **Aislamiento Multi-Tenant:** Toda consulta a Qdrant/PostgreSQL DEBE incluir el filtro `WHERE (tenant_id == CurrentTenantID OR tenant_id == 'GLOBAL')`.
- **Protocolo Dry-Run:** Acciones con efectos secundarios deben retornar `DryRunResult` con tokens dinámicos `dryrun_<uuid>`.

## 2. Uso Mandatorio de MCP (`codebase-memory-mcp`)
Si el servidor MCP `codebase-memory-mcp` está disponible, utilizar preferentemente `search_graph`, `trace_path` y `get_code_snippet`.

## 3. Workflow Mandatorio de Calidad y Linters (Post-Code Quality Gate)
Antes de dar por completado cualquier cambio de código en Go, el asistente DEBE ejecutar automáticamente:
```bash
go vet ./...
gosec ./...
go test -v ./...
```
Si se detecta cualquier vulnerabilidad o advertencia, DEBE corregirse de inmediato.

## 4. Reglas de Codificación en Go
- **Rendimiento Sub-Milisegundo:** Código idiomático en Go 1.22+ sin dependencias frágiles.
- **Manejo Estricto de Errores:** NUNCA ignorar errores devueltos (`if err != nil`). Retornar o envolver errores con contexto mediante `fmt.Errorf("contexto: %w", err)`.
- **Cierre Seguro de Recursos:** Garantizar `defer file.Close()`, `defer resp.Body.Close()`.
