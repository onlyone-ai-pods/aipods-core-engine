# 📜 SPEC: AI Pod - SAP Enterprise (S/4HANA/ECC) & SAP Business One Integrator
**ID:** SPEC-POD-06  
**Épica Relacionada:** Épica 12 (AI Pod - SAP Enterprise & SAP Business One Integration)  
**Estado:** PROPOSED / SPEC-DRIVEN  

---

## 1. Visión General & Comparativa Tecnológica (Odoo vs SAP)

Este AI Pod especializado otorga a la plataforma la capacidad de interactuar con ecosistemas **SAP (SAP S/4HANA, SAP ECC, SAP R/3 y SAP Business One)**. 

A diferencia de Odoo (que utiliza principalmente XML-RPC o JSON-RPC), SAP ofrece múltiples métodos de conexión según la arquitectura desplegada en la empresa cliente.

---

## 2. Métodos de Conexión Soportados por el Pod SAP

### 2.1 SAP Gateway (OData RESTful) - *Estándar Moderno para S/4HANA*
* **Mecanismo:** Peticiones HTTP RESTful (GET, POST, PUT, DELETE) consumiendo servicios OData.
* **Formato:** JSON y XML.
* **Librería en Python:** `requests` / `httpx`.

### 2.2 SOAP Web Services (WSDL) - *Equivalente Tecnológico Estricto*
* **Mecanismo:** Envío de payloads XML sobre HTTP basados en contratos WSDL.
* **Formato:** XML estricto.
* **Librería en Python:** `zeep`.

### 2.3 RFC (Remote Function Call) & BAPI - *Conexión Nativa Profunda*
* **Mecanismo:** Protocolo propietario TCP/IP consumiendo Business Application Programming Interfaces (BAPIs).
* **Formato:** Binario / Estructuras C.
* **Librería en Python:** `PyRFC` (requiere SAP NW RFC SDK).

### 2.4 SAP Business One Service Layer - *Para PYMES*
* **Mecanismo:** API REST basada en OData (JSON).
* **Librería en Python:** `requests`.

---

## 3. Matriz Comparativa para Desarrolladores

| Característica | Odoo ERP | SAP S/4HANA / ECC | SAP Business One (B1) |
| :--- | :--- | :--- | :--- |
| **Protocolo Principal** | XML-RPC / JSON-RPC | OData (REST) / RFC / SOAP | Service Layer (REST/OData) |
| **Formato de datos** | XML / JSON | JSON / XML / Binario (RFC) | JSON |
| **Complejidad de Conexión** | Muy baja (Nativa) | Media/Alta (Configuración Gateway) | Baja (Service Layer) |
| **Librería Python Común** | `xmlrpc.client` (nativa) | `requests` (OData) / `PyRFC` / `zeep` | `requests` |

---

## 4. Herramientas del Pod (Tools Schema)

### 4.1 Tool `consultar_sap_odata`
```json
{
  "name": "consultar_sap_odata",
  "description": "Consulta entidades o datos en SAP S/4HANA mediante SAP Gateway OData REST API.",
  "parameters": {
    "type": "object",
    "properties": {
      "service_path": { "type": "string", "example": "/sap/opu/odata/sap/API_BUSINESS_PARTNER" },
      "entity_set": { "type": "string", "example": "A_BusinessPartner" },
      "filter_query": { "type": "string", "example": "SearchTerm1 eq 'DEMO'" },
      "dry_run": { "type": "boolean", "default": true }
    },
    "required": ["service_path", "entity_set", "dry_run"]
  }
}
```

### 4.2 Tool `ejecutar_bapi_pyrfc`
```json
{
  "name": "ejecutar_bapi_pyrfc",
  "description": "Ejecuta una función BAPI nativa en SAP ECC/R3 mediante PyRFC.",
  "parameters": {
    "type": "object",
    "properties": {
      "bapi_name": { "type": "string", "example": "BAPI_CUSTOMER_GETDETAIL2" },
      "import_params": { "type": "object" },
      "dry_run": { "type": "boolean", "default": true }
    },
    "required": ["bapi_name", "dry_run"]
  }
}
```

---

## 5. Escenario BDD de Integración SAP OData

```gherkin
Given un usuario solicitando al Pod SAP "Consulta el socio de negocio con CUIT 30-71123456-8 en SAP S/4HANA"
When el AI Pod procesa la solicitud
Then el Pod DEBE invocar la herramienta consultar_sap_odata con dry_run=true
And construir la consulta OData $filter correspondiente
And presentar la simulación con el endpoint HTTP REST esperado antes de la ejecución real
```
