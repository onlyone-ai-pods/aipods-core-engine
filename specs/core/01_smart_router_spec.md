# 📜 SPEC: Smart Router & Intent Classifier
**ID:** SPEC-CORE-01  
**Épica Relacionada:** Épica 5 (HU 5.1)  
**Estado:** DRAFT / READY FOR IMPL  

---

## 1. Alcance y Requisitos
El Smart Router es el componente de orquestación encargado de evaluar la intención del usuario y seleccionar el AI Pod especializado o coordinar múltiples AI Pods si la consulta es multidisciplinaria.

---

## 2. Contrato de Interfaz (Schema)

### 2.1 Router Input Payload
```json
{
  "$schema": "http://json-schema.org/draft-07/schema#",
  "type": "object",
  "properties": {
    "user_prompt": { "type": "string", "minLength": 1 },
    "tenant_id": { "type": "string" },
    "conversation_history": {
      "type": "array",
      "items": {
        "type": "object",
        "properties": {
          "role": { "type": "string", "enum": ["user", "assistant"] },
          "content": { "type": "string" }
        }
      }
    }
  },
  "required": ["user_prompt", "tenant_id"]
}
```

### 2.2 Router Decision Output Payload
```json
{
  "type": "object",
  "properties": {
    "is_multi_domain": { "type": "boolean" },
    "target_pods": {
      "type": "array",
      "items": {
        "type": "string",
        "enum": ["AFIP_FINANCE", "EVOCRM_HELPDESK", "SOCIAL_MARKETING", "SCM_LOGISTICS"]
      }
    },
    "sub_queries": {
      "type": "array",
      "items": {
        "type": "object",
        "properties": {
          "pod": { "type": "string" },
          "extracted_query": { "type": "string" }
        }
      }
    },
    "confidence_score": { "type": "number", "minimum": 0.0, "maximum": 1.0 }
  },
  "required": ["is_multi_domain", "target_pods", "confidence_score"]
}
```

---

## 3. Escenarios de Comportamiento (BDD - Given / When / Then)

### Escenario 1: Enrutamiento directo a Pod AFIP
```gherkin
Given un mensaje de usuario: "¿Cómo genero el archivo CSR para AFIP en Ubuntu?"
And un tenant_id válido "tenant_1001"
When el Smart Router procesa la intención del mensaje
Then el campo is_multi_domain debe ser false
And el arreglo target_pods debe contener exactamente ["AFIP_FINANCE"]
And confidence_score debe ser >= 0.90
```

### Escenario 2: Enrutamiento a Pod SCM (Landed Costs)
```gherkin
Given un mensaje de usuario: "Necesito prorratear el costo de flete internacional en una recepción de compras"
When el Smart Router procesa la intención del mensaje
Then el arreglo target_pods debe contener ["SCM_LOGISTICS"]
And el sub_queries extraído debe centrarse en "Landed Costs y prorrateo de compras"
```

### Escenario 3: Enrutamiento Multidominio (Compra + Factura AFIP)
```gherkin
Given un mensaje de usuario: "Tengo un problema al crear la orden de compra en WMS y cuando intento facturarla me tira error de CUIT en AFIP"
When el Smart Router procesa la intención del mensaje
Then el campo is_multi_domain debe ser true
And target_pods debe contener ["SCM_LOGISTICS", "AFIP_FINANCE"]
And sub_queries debe contener 2 elementos mapeando las dudas específicas a cada Pod
```

---

## 4. Invariantes & Aserciones de Rendimiento
* **Latencia del Router:** $\le 350\text{ms}$ utilizando modelos rápidos (`Claude 3 Haiku` o `GPT-4o-mini`).
* **Fallback Rule:** Si `confidence_score < 0.60`, derivar a `AFIP_FINANCE` por defecto con una pregunta aclaratoria al usuario.
