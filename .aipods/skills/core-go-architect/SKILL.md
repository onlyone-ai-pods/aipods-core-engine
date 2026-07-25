---
name: core-go-architect
description: Guía experta de arquitectura en Go 1.22+ para el motor backend de AI Pods Enterprise.
---

# 🚀 Skill: Core Go Architect

Esta habilidad instruye a asistentes de IA para desarrollar el backend del proyecto `aipods-core-engine` en **Golang 1.22+**:

## 1. Trazabilidad Estricta con Especificaciones SDD
Antes de escribir o modificar cualquier archivo Go, consultar la especificación ejecutable en `specs/` correspondiente. El código debe implementar exactamente los esquemas JSON de las herramientas, el protocolo `dry_run = true` y los escenarios BDD definidos en las specs.

## 2. Uso Mandatorio de MCP (`codebase-memory-mcp`)
Si el servidor MCP `codebase-memory-mcp` está disponible, utilizar preferentemente `search_graph`, `trace_path` y `get_code_snippet` antes de hacer búsquedas manuales con grep/glob.

## 3. Workflow Mandatorio de Calidad y Linters (Post-Code Gate)
Antes de dar por completado cualquier cambio de código en Go, el asistente DEBE ejecutar automáticamente:
```bash
go vet ./...
/home/martin/go/bin/gosec ./...
go test -v ./...
```
Si se detecta cualquier vulnerabilidad o advertencia, DEBE corregirse de inmediato.

## 4. Reglas de Codificación en Go
- **Rendimiento Sub-Milisegundo:** Código idiomático en Go 1.22+ sin dependencias frágiles.
- **Manejo Estricto de Errores:** NUNCA ignorar errores devueltos (`if err != nil`). Retornar o envolver errores con contexto mediante `fmt.Errorf("contexto: %w", err)`.
- **Cierre Seguro de Recursos:** Garantizar `defer file.Close()`, `defer resp.Body.Close()`.
