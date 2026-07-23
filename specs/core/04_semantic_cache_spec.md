# 📜 SPEC: Semantic Cache & Async Queues
**ID:** SPEC-CORE-04  
**Épica Relacionada:** Épica 9 (HU 9.1, HU 9.2)  
**Estado:** DRAFT / READY FOR IMPL  

---

## 1. Alcance
Especificaciones para la optimización de latencia y costo de API mediante caché semántico en Redis y delegación de tareas pesadas a colas Celery/RabbitMQ.

---

## 2. Parámetros de Caché Semántico

* **Vector Store de Caché:** Redis Index con distancia Cosine.
* **Umbral de Similitud ($\text{Similarity Threshold}$):** $0.95$ ($95\%$).
* **TTL (Time to Live):** 86,400 segundos (24 horas).
* **Scope del Caché:** Isolado por `tenant_id` (Caché por tenant + Caché Global).

---

## 3. Escenarios de Comportamiento (BDD)

### Escenario 1: Impacto en Caché Semántico (Cache Hit)
```gherkin
Given que la pregunta "pasos para crear certificado AFIP" fue consultada previamente y guardada en caché
When un usuario del mismo tenant pregunta "¿cuáles son los pasos para generar el certificado de AFIP?"
And la similitud vectorial entre los embeddings de ambas preguntas es 0.97 (>= 0.95)
Then el sistema debe responder directamente desde Redis en < 100ms
And NO se debe realizar ninguna llamada a la API del LLM (0 costo de tokens)
And la respuesta debe marcar "cached": true
```

### Escenario 2: Procesamiento Asíncrono de Balances Extensos
```gherkin
Given un usuario subiendo un archivo PDF de balance con más de 30 páginas
When se solicita la asistencia financiera mensual
Then el API Gateway debe delegar el procesamiento a la cola de RabbitMQ
And retornar inmediatamente una respuesta HTTP 202 Accepted con un task_id
And el worker de Celery debe procesar el documento en segundo plano sin bloquear el sistema
```
