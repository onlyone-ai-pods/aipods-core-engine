package main

import (
	"fmt"
	"net/http"
	"os"

	"github.com/gin-gonic/gin"
)

type ChatRequest struct {
	TenantID string `json:"tenant_id" binding:"required"`
	Message  string `json:"message" binding:"required"`
}

type ChatResponse struct {
	PodID      string   `json:"pod_id"`
	Answer     string   `json:"answer"`
	Citations  []string `json:"citations"`
	IsDryRun   bool     `json:"is_dry_run"`
	Status     string   `json:"status"`
}

func main() {
	r := gin.Default()

	// Healthcheck Endpoint
	r.GET("/healthz", func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{
			"status":  "healthy",
			"version": "3.3.0",
		})
	})

	// Sprint 1 MVP: Chat Endpoint matching specs/pods/01_afip_finance_spec.md
	r.POST("/api/v1/chat/completions", func(c *gin.Context) {
		var req ChatRequest
		if err := c.ShouldBindJSON(&req); err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
			return
		}

		// Sprint 1 Mock Response for AFIP Certificate Query
		res := ChatResponse{
			PodID:  "POD_AFIP_FINANCE",
			Answer: "Para generar la clave privada y el archivo CSR para AFIP en Odoo, ejecute el siguiente comando OpenSSL en su terminal:\n\n```bash\nopenssl req -new -key privada.key -out pedido.csr\n```\n\nPosteriormente, cargue el certificado `.crt` emitido por AFIP en la configuración de la compañía en Odoo.",
			Citations: []string{
				"Guia_AFIP_Certificados_v1.pdf (Pagina 4)",
			},
			IsDryRun: true,
			Status:   "SUCCESS",
		}

		c.JSON(http.StatusOK, res)
	})

	port := os.Getenv("PORT")
	if port == "" {
		port = "8080"
	}

	fmt.Printf("🚀 AI Pods Engine Server running on port %s\n", port)
	if err := r.Run(":" + port); err != nil {
		fmt.Printf("Server failed: %v\n", err)
	}
}
