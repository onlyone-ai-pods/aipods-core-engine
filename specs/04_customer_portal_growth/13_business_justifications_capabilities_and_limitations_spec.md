# 📜 SPEC: Justificaciones de Negocio, Prestaciones Avanzadas y Limitaciones de la Plataforma
**ID:** SPEC-CORE-13  
**Épica Relacionada:** Estrategia de Negocio, ROI, Límites Técnicos & Portal de Clientes  
**Estado:** PROPOSED / SPEC-DRIVEN  

---

## 1. Visión General

Esta especificación consolida las **Justificaciones de Negocio (Retorno de Inversión / ROI)**, las **Prestaciones Avanzadas** y las **Limitaciones Técnicas y Operativas** de la plataforma AI Pods. Su objetivo es nutrir la comunicación del Portal de Clientes y guiar las expectativas comerciales y de arquitectura.

---

## 2. Justificaciones de Negocio & Retorno de Inversión (ROI)

Más allá de automatizar consultas simples, el proyecto AI Pods se fundamenta en **4 Justificaciones de Negocio de Alto Impacto**:

### 2.1 Preservación y Transferencia del Conocimiento Crítico (Anti-Brain Drain)
* **El Problema:** Cuando un consultor Senior de Odoo, AFIP o Logística SCM renuncia o se ausenta, el conocimiento táctico de cómo resolver configuraciones complejas se pierde o detiene la operación.
* **El Valor AI Pods:** El conocimiento queda "clonado" e institucionalizado en la empresa. El tiempo de *onboarding* de nuevos consultores o empleados de clientes se reduce de meses a pocos días.

### 2.2 Reducción drástica del Costo y Tiempo de Soporte Nivel 1 y 2
* **El Problema:** Más del 70% de los tickets de soporte en Odoo (ej. generación de claves AFIP, tokens vencidos en Meta, reglas de reabastecimiento en WMS) son repetitivos y consumen horas de consultores costosos.
* **El Valor AI Pods:** Respuestas instantáneas 24/7/365 en $<3$ segundos con citas textuales verificadas, reduciendo el costo por ticket hasta en un **80%**.

### 2.3 Estandarización de Respuestas & Prevención de Errores Humanos
* **El Problema:** Consultores distintos suelen ofrecer respuestas diferentes o desactualizadas para la misma norma legal o procedimiento.
* **El Valor AI Pods:** Respuestas estandarizadas basadas 100% en la documentación validada por los socios Seniors, eliminando la discrepancia de criterios.

### 2.4 Operación Omnicanal e Integración Directa en el Trabajo
* **El Problema:** El cliente debe interrumpir su trabajo para abrir tickets en portales externos y esperar horas de oficina.
* **El Valor AI Pods:** Asistencia contextual e instantánea desde WhatsApp (vía EvoCRM), Email (vía Amazon SES), o directamente en su ERP (Odoo/SAP).

---

## 3. Prestaciones Avanzadas de la Plataforma (High-Tier Capabilities)

1. **Orquestación Multi-Agente Autónoma:** El Smart Router puede descomponer una consulta compleja (ej. *"Compré un insumo importado y cuando quise facturarlo me dio error AFIP"*) y consultar en paralelo a los Pods SCM y AFIP para consolidar una respuesta unificada.
2. **Generación de Código e Instrucciones Exactas:** Generación de scripts OpenSSL para terminales, validación de estructuras JSON de Webhooks e instrucciones paso a paso para rutas Push/Pull en WMS.
3. **Análisis Financiero de Balances Extensos:** Procesamiento asíncrono no bloqueante de balances en PDF/CSV de más de 50 páginas con diagnósticos en lenguaje natural.
4. **Data CI/CD con Conmutación Atómica:** Actualización instantánea de normativas sin tiempo de inactividad mediante la conmutación de metadatos `status: ACTIVE`.

---

## 4. Limitaciones Técnicas y Operativas (Platform Boundaries)

Para mantener la transparencia con los clientes y evitar falsas expectativas, la plataforma establece **4 Limitaciones Estrictas**:

| Limitación Operativa | Explicación & Frontera Técnica | Mitigación / Regla de Arquitectura |
| :--- | :--- | :--- |
| **1. No Reemplaza la Firma o Responsabilidad Legal/Contable** | El AI Pod asesora y guía técnicamente, pero la responsabilidad fiscal de la presentación ante AFIP/ARCA o la firma de estados contables sigue perteneciendo al profesional humano. | **Disclaimer Explícito:** Advertencia automática en respuestas de carácter fiscal/financiero. |
| **2. Calidad Dependiente de la Fuente (Garbage In, Garbage Out)** | Si la documentación provista por el consultor Senior contiene datos contradictorios o desactualizados, el motor RAG no puede corregir la fuente original. | **Evaluación pre-ingesta (Evals Gate)** y versión de documentos. |
| **3. Human-in-the-Loop para Acciones Críticas** | El AI Pod no ejecuta escrituras o cambios destructivos en las bases de producción de Odoo o envíos masivos sin aprobación explícita. | **Modo Asistivo:** El Pod propone los comandos/acciones y el usuario confirma la ejecución. |
| **4. Procesamiento Asíncrono para Archivos Masivos** | Consultas en tiempo real deben responder en $<3,000\text{ms}$. Archivos extensos ($>50$ páginas o $>10\text{MB}$) deben ser procesados obligatoriamente vía NATS/Celery en segundo plano. | **Notificación Push / Async Status:** Retorno de HTTP 202 Accepted. |

---

## 5. Escenario BDD de Transparencia de Limitaciones

```gherkin
Given un usuario solicitando al Pod AFIP "Presenta mi declaración jurada mensual en el portal de AFIP automáticamente"
When el AI Pod procesa la solicitud
Then el Pod DEBE responder indicando que por limitaciones de seguridad y responsabilidad legal no ejecuta presentaciones fiscales finales
And debe proveer la guía paso a paso con los datos calculados para que el usuario humano realice la presentación
```
