# 🚀 Proyecto Antigravity: AI Pods Enterprise SaaS Platform

Plataforma SaaS basada en arquitectura RAG (Retrieval-Augmented Generation) diseñada para "clonar" e institucionalizar el conocimiento experto de consultores y especialistas empresariales en AFIP/ARCA, EvoCRM, Odoo, SAP, Salesforce y Cadena de Suministros. El sistema se compone de múltiples "AI Pods" (Agentes especializados agnósticos conectables a cualquier API/ERP) orquestados por un Router inteligente, escalable para soportar +200 empresas bajo un modelo multi-tenant seguro.

---

## 📌 Epicas Consolidadas

### 👑 ÉPICA 1: AI Pod - AFIP / ARCA & Balances Financieros
*Automatización de asistencia técnica burocrática, normativas fiscales y reportes financieros.*

* **HU 1.1 - Generación y Validación de Claves AFIP:**
  > **Como** consultor o usuario empresarial,  
  > **quiero** consultar al AI Pod cómo generar la clave privada y archivo CSR para AFIP en mi sistema,  
  > **para** obtener el comando OpenSSL exacto y la guía paso a paso sin cometer errores de formato.

* **HU 1.2 - Diagnóstico de Balances Financieros:**
  > **Como** analista financiero o gerente,  
  > **quiero** subir mis reportes de balance exportados en PDF o CSV al AI Pod,  
  > **para** recibir un análisis estructurado de liquidez, solvencia y EBITDA en lenguaje natural.

---

### 👑 ÉPICA 2: AI Pod - EvoCRM & Helpdesk Integrado
*Asistencia omnicanal y gestión de tickets de soporte conectada a WhatsApp y ERPs.*

* **HU 2.1 - Integración EvoCRM y Helpdesk:**
  > **Como** especialista de soporte,  
  > **quiero** consultar cómo configurar webhooks y credenciales entre Helpdesk y EvoCRM (WhatsApp),  
  > **para** resolver problemas de tokens o webhooks caídos instantáneamente.

---

### 👑 ÉPICA 3: AI Pod - Social Marketing & Omnicanalidad
*Diagnóstico y configuración de APIs de Meta, Instagram y campañas de marketing.*

* **HU 3.1 - Troubleshooting de API de Meta/Instagram:**
  > **Como** responsable de marketing digital,  
  > **quiero** consultar la causa por la que se desconectó el token de la cuenta de Instagram/Meta,  
  > **para** obtener la solución paso a paso y re-vincular la cuenta sin perder leads.

---

### 👑 ÉPICA 4: AI Pod - Cadena de Suministros (SCM, WMS, MRP, Compras)
*Optimización logística, reglas de reabastecimiento e imputación de Landed Costs en ERPs.*

* **HU 4.1 - Reglas de Reabastecimiento e Inventario (WMS/MRP):**
  > **Como** jefe de depósito o logística,  
  > **quiero** consultar la mejor estrategia para configurar puntos de pedido min/max en el ERP,  
  > **para** obtener una guía paso a paso de cómo configurar reglas Push/Pull y ubicaciones.

* **HU 4.2 - Importaciones y Landed Costs:**
  > **Como** analista de compras internacionales,  
  > **quiero** prorratear gastos de flete y aranceles de importación sobre un embarque,  
  > **para** que el costo estándar/promedio refleje el valor real de importación en el ERP.

---

### 👑 ÉPICA 5: Smart Router & Clasificador de Intenciones
*Orquestación inteligente entre AI Pods.*

### 👑 ÉPICA 6: RAG Engine & Vector DB Pipeline
*Ingesta, vectorización y citación de documentos.*

### 👑 ÉPICA 7: Invalidador de Caché y Notificaciones Event-Driven
*Sincronización en tiempo real vía Redis Pub/Sub y NATS JetStream.*

### 👑 ÉPICA 8: Aislamiento Multi-Tenant & Seguridad de Metadatos
*Garantía estricta de separación de datos por tenant_id.*

### 👑 ÉPICA 9: Caché Semántico Redis Active-Active
*Respuestas instantáneas <100ms y failover multi-región.*

### 👑 ÉPICA 10: Ingesta Asíncrona de Balances Extensos
*Procesamiento no bloqueante de archivos masivos vía NATS JetStream.*
