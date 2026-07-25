# 🧠 Regla: Uso Prioritario de codebase-memory-mcp

Si el servidor MCP `codebase-memory-mcp` está presente y cargado en el entorno del asistente de IA, el agente **DEBE PRIORIZAR OBLIGATORIAMENTE** las herramientas de grafo de conocimiento sobre búsquedas tradicionales con grep/glob para el descubrimiento e inspección de código.

## Orden de Prioridad de Herramientas MCP
1. `search_graph`: Buscar funciones, clases, rutas, componentes y variables por patrón de nombre.
2. `trace_path`: Trazar quién llama a una función o qué funciones son invocadas por ella.
3. `get_code_snippet`: Leer el código fuente exacto de un símbolo o función específica.
4. `query_graph`: Ejecutar consultas Cypher para patrones complejos de arquitectura.
5. `get_architecture`: Obtener el resumen de arquitectura de alto nivel del proyecto.

## Casos de Uso para Fallback a Grep/Glob
Utilizar grep o glob únicamente para:
- Búsqueda de literales de texto plano, mensajes de error o valores en archivos de configuración (`.json`, `.yaml`, `.env`).
- Archivos fuera del código fuente Go/React (Dockerfiles, shell scripts, Markdown).
- Cuando las herramientas MCP retornen resultados insuficientes.
