---
name: core-go-architect
description: Guía experta de arquitectura en Go 1.22+ para el motor backend de AI Pods Enterprise.
---

# 🚀 Skill: Core Go Architect

Esta habilidad instruye a asistentes de IA para desarrollar el backend del proyecto `aipods-core-engine` en **Golang 1.22+**:

## Reglas de Codificación en Go

1. **Rendimiento y Latencia Sub-Milisegundo:**
   - Escribir código idioomático en Go 1.22+ sin sobre-ingeniería ni dependencias frágiles.
   - Utilizar Gin Web Framework para endpoints REST HTTP.

2. **Manejo Estricto de Errores:**
   - NUNCA ignorar errores devueltos (`if err != nil`). Retornar o envolver errores con contexto mediante `fmt.Errorf("contexto: %w", err)`.
   - Garantizar el cierre seguro de recursos con `defer file.Close()`, `defer resp.Body.Close()`.

3. **Verificación de Linters:**
   - Todo código debe pasar `golangci-lint` y `gosec` sin advertencias de seguridad antes de abrir un Code PR.
