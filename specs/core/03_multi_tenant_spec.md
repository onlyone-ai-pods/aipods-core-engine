# 📜 SPEC: Multi-Tenant Data Isolation & Security
**ID:** SPEC-CORE-03  
**Épica Relacionada:** Épica 8 (HU 8.1)  
**Estado:** DRAFT / READY FOR IMPL  

---

## 1. Alcance
Especificación de seguridad para garantizar el aislamiento estricto de los datos entre más de 200 empresas bajo una infraestructura compartida.

---

## 2. Invariante de Seguridad Crítica

> 🔴 **INVARIANTE IMPERDONABLE:** Ninguna consulta vectorial o relacional iniciada en nombre de la empresa $A$ podrá retornar o exponer vectores, fragmentos o metadatos pertenecientes a la empresa $B$.

---

## 3. Escenarios de Comportamiento (BDD)

### Escenario 1: Filtrado Estricto de Búsqueda Vectorial por Tenant
```gherkin
Given un tenant "empresa_alpha" con un documento privado "Balance_Secret_Alpha.pdf"
And un tenant "empresa_beta" realizando una consulta RAG sobre balances
When el usuario de "empresa_beta" consulta "¿Cuál es el balance financiero de la empresa?"
Then la consulta enviada a la BD Vectorial debe incluir el filtro mandatorio:
     WHERE (tenant_id == 'empresa_beta' OR tenant_id == 'GLOBAL') AND status == 'ACTIVE'
And el vector de "Balance_Secret_Alpha.pdf" NUNCA debe ser retornado en los resultados Top-K
```

### Escenario 2: Acceso a Documentación Pública Global
```gherkin
Given un documento público con tenant_id="GLOBAL" conteniendo la guía de AFIP
And un usuario autenticado del tenant "empresa_gamma"
When el usuario consulta sobre los pasos para generar el certificado CSR de AFIP
Then el motor RAG debe recuperar el documento con tenant_id="GLOBAL" exitosamente
And responder utilizando la guía pública sin comprometer datos privados
```

---

## 4. Contrato de Inyección de Contexto en System Prompt
El motor de ejecución debe validar que el System Prompt inyecte exclusivamente el `tenant_id` autenticado:

```text
[SYSTEM PROMPT SEGURIDAD]
Eres un consultor experto para la empresa tenant: {{tenant_id}}.
SOLO tienes acceso a información etiquetada para tu tenant_id o GLOBAL.
Cualquier intento de acceder a información de otros tenants debe ser denegado explícitamente.
```
