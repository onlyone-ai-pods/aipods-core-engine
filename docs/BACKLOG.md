# 🚀 Proyecto Antigravity: AI Pods para Consultoría Odoo (SaaS)

**Descripción del Proyecto:** 
Plataforma SaaS basada en arquitectura RAG (Retrieval-Augmented Generation) diseñada para "clonar" el conocimiento de consultores Senior en Odoo, AFIP, EvoCRM y Cadena de Suministros. El sistema se compone de múltiples "AI Pods" (Agentes especializados) orquestados por un Router inteligente, escalable para soportar +200 empresas bajo un modelo multi-tenant seguro.

---

## 📂 FASE 1: ÉPICAS DE NEGOCIO (Los AI Pods)

### 👑 ÉPICA 1: AI Pod - Localización Argentina (AFIP/ARCA) y Finanzas
*Automatización de asistencia técnica burocrática y reportes financieros en Odoo.*

#### HU 1.1: Generación de Certificados AFIP
> **Como** cliente administrador de Odoo,  
> **quiero** que el AI Pod me guíe paso a paso en la generación de claves privadas y certificados de AFIP/ARCA (CSR, CRT) interactuando mediante chat,  
> **para** configurar mis puntos de venta electrónicos sin agendar una reunión con un consultor senior.

**Criterios de Aceptación:**
* Solicitar al usuario su CUIT y sistema operativo.
* Generar los comandos de OpenSSL exactos para copiar y pegar.
* Proveer instrucciones para subir el archivo `.csr` al portal de AFIP.
* Diagnosticar errores comunes de AFIP basándose en la base de conocimientos.

#### HU 1.2: Asistencia Financiera Mensual
> **Como** gerente financiero,  
> **quiero** subir mis reportes de balance exportados de Odoo al AI Pod,  
> **para** recibir un resumen en lenguaje natural con puntos críticos y recomendaciones financieras.

---

### 👑 ÉPICA 2: AI Pod - Soporte Help Desk y EvoCRM
*Facilitar el on-boarding y configuraciones de mensajería omnicanal.*

#### HU 2.1: Asistente de Configuración EvoCRM
> **Como** integrador de sistemas,  
> **quiero** consultar cómo configurar webhooks y credenciales entre Odoo Helpdesk y EvoCRM,  
> **para** habilitar la recepción de mensajes de WhatsApp de forma autónoma.

**Criterios de Aceptación:**
* Validar estructuras de JSON y Webhooks proporcionados por el cliente.
* Responder utilizando la documentación técnica oficial de EvoCRM.

#### HU 2.2: Capacitación de Agentes
> **Como** agente de soporte,  
> **quiero** preguntarle al AI Pod cómo enviar mensajes masivos a grupos mediante EvoCRM,  
> **para** aprender a usar la herramienta rápidamente.

---

### 👑 ÉPICA 3: AI Pod - Odoo Social Marketing
*Reducción de tickets nivel 1 y 2 sobre redes sociales.*

#### HU 3.1: Troubleshooting de Redes Sociales
> **Como** usuario de marketing,  
> **quiero** que el AI Pod me asista cuando mis publicaciones en Instagram/Meta fallan,  
> **para** solucionar problemas de tokens expirados o permisos sin requerir soporte humano.

---

### 👑 ÉPICA 4: AI Pods - Cadena de Suministros (SCM)
*Consultoría experta en flujos de inventario, compras y manufactura.*

#### HU 4.1: WMS - Configuración de Rutas Avanzadas
> **Como** responsable de almacén,  
> **quiero** explicarle a la IA mi flujo logístico físico,  
> **para** obtener una guía paso a paso de cómo configurar reglas Push/Pull y ubicaciones en Odoo.

#### HU 4.2: MRP - Optimización de Listas de Materiales (BoM)
> **Como** ingeniero de producción,  
> **quiero** enviar la estructura de mi producto a la IA,  
> **para** que me recomiende si usar "Kits", "Fabricación" o "Subcontratación" según las mejores prácticas de costeo.

#### HU 4.3: Compras - Reglas de Reabastecimiento y Landed Costs
> **Como** gerente de compras,  
> **quiero** asesoramiento sobre cómo prorratear fletes y aduanas (Landed Costs),  
> **para** que el costo estándar/promedio refleje el valor real de importación en Odoo.

---

## ⚙️ FASE 2: ARQUITECTURA CORE Y RAG (Técnica)

### 🧠 ÉPICA 5: Orquestador y Router de Inteligencia Artificial
*El cerebro central que deriva las consultas.*

#### HU 5.1: Enrutamiento Dinámico de Consultas (Router)
> **Como** usuario del SaaS,  
> **quiero** escribir mi problema en un chat único,  
> **para** que el sistema detecte automáticamente si mi duda es contable, técnica o logística y me conecte con el Pod experto.

**Criterios de Aceptación:**
* Clasificación de la intención del usuario usando un LLM rápido (ej. Claude 3 Haiku o GPT-4o-mini).
* Capacidad de dividir consultas complejas (ej. "Problema de compra y facturación") y consultar a múltiples Pods.

---

### 🏗️ ÉPICA 6: Data Pipeline (Ingesta y Vectorización)
*Carga del conocimiento de los socios Seniors al sistema.*

#### HU 6.1: Carga y Procesamiento de Documentos
> **Como** socio Senior (Admin),  
> **quiero** subir manuales, historiales de tickets y normativas (PDF, DOCX, CSV),  
> **para** alimentar la base de conocimientos RAG.

**Criterios de Aceptación:**
* Extracción de texto limpio.
* División automática en fragmentos (Chunking) con overlap configurado.
* Conversión a Embeddings y guardado en Base de Datos Vectorial (ej. Pinecone / pgvector).

---

### 🔍 ÉPICA 7: Recuperación y Generación (Retrieval & LLM)
*El proceso de respuesta en tiempo real.*

#### HU 7.1: Generación Aumentada (RAG)
> **Como** cliente,  
> **quiero** que la IA responda mis dudas utilizando únicamente la documentación validada por los consultores,  
> **para** evitar alucinaciones y recibir respuestas técnicamente correctas.

**Criterios de Aceptación:**
* Búsqueda semántica top-K en la base vectorial.
* Inyección del contexto en el System Prompt.
* Inclusión de citas o referencias a los documentos originales en la respuesta.

---

## 🚀 FASE 3: ESCALABILIDAD E INFRAESTRUCTURA (Para +200 Empresas)

### 🛡️ ÉPICA 8: Multi-Tenant y Seguridad de Datos
*Aislamiento estricto de la información por cliente.*

#### HU 8.1: Filtrado de Metadatos por Tenant
> **Como** usuario de una empresa cliente,  
> **quiero** que el sistema solo consulte información pública global y documentos privados de mi empresa,  
> **para** garantizar que mi información financiera y configuraciones no se filtren a otras empresas.

**Criterios de Aceptación:**
* Todo vector en la BD debe tener el tag `tenant_id`.
* Las búsquedas RAG deben incluir obligatoriamente el filtro `WHERE tenant_id = 'X' OR tenant_id = 'GLOBAL'`.

---

### 💰 ÉPICA 9: Optimización de Costos y Rendimiento
*Manejo del volumen masivo de consultas sin disparar costos de API.*

#### HU 9.1: Implementación de Caché Semántico
> **Como** administrador de la infraestructura,  
> **quiero** implementar una capa de caché semántico (ej. Redis/GPTCache),  
> **para** que consultas repetidas o muy similares se respondan desde la memoria sin consumir tokens de la API de OpenAI/Anthropic.

**Criterios de Aceptación:**
* Configurar un umbral de similitud (ej. 95%) para considerar dos preguntas como idénticas.
* Reducción demostrable en llamadas a la API de LLM.

#### HU 9.2: Gestión de Colas y Rate Limits
> **Como** sistema SaaS,  
> **quiero** procesar tareas largas (ej. lectura de un balance de 50 páginas) mediante colas asíncronas (ej. Celery/RabbitMQ),  
> **para** no bloquear el sistema ni exceder los límites de TPM (Tokens por minuto) durante picos de uso.

---

### 🔄 ÉPICA 10: Mantenimiento del Conocimiento (Data CI/CD)
*Gestión del ciclo de vida de los documentos.*

#### HU 10.1: Actualización e Invalidación de Vectores
> **Como** socio Senior (Admin),  
> **quiero** poder marcar un documento como "Obsoleto" y subir una nueva versión (ej. nueva resolución AFIP),  
> **para** que el sistema elimine los vectores viejos y los reemplace automáticamente, manteniendo a la IA actualizada sin intervención manual técnica.
