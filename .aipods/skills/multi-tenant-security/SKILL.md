---
name: multi-tenant-security
description: Reglas y salvaguardas estrictas para el aislamiento multi-tenant y seguridad RS256 en Go.
---

# 🔒 Skill: Multi-Tenant Security & Isolation

Esta habilidad enforza las reglas de seguridad multi-tenant en todas las consultas y componentes en Go:

## Invariantes de Seguridad Obligatorias

1. **Filtrado Obligatorio por Metadata `tenant_id`:**
   Toda consulta a PostgreSQL o Qdrant DEBE incluir obligatoriamente la cláusula:
   ```sql
   WHERE (tenant_id = CurrentTenantID OR tenant_id = 'GLOBAL') AND status = 'ACTIVE'
   ```
   NUNCA realizar consultas globales sin el contexto del tenant extraído desde el token JWT o el contexto HTTP.

2. **Protocolo Dry-Run Obligatorio:**
   Toda herramienta con efectos secundarios debe validar `dry_run = true` por defecto antes de mutar la base de datos o enviar llamados a APIs externas.

3. **Verificación de Firma JWT RS256:**
   Autenticación validada con clave pública RSA de 2048+ bits (RS256).
