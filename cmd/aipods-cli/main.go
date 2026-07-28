package main

import (
	"flag"
	"fmt"
	"os"
	"strings"

	"github.com/martinllanos/only-ai-pods/internal/cli/audit"
	"github.com/martinllanos/only-ai-pods/internal/cli/register"
	"github.com/martinllanos/only-ai-pods/internal/cli/scaffold"
	"github.com/martinllanos/only-ai-pods/internal/cli/validator"
)

const version = "23.0.0"

func main() {
	if len(os.Args) < 2 {
		printUsage()
		os.Exit(1)
	}

	command := os.Args[1]

	switch command {
	case "version", "--version", "-v":
		fmt.Printf("aipods-cli version %s (AI Pods Enterprise SaaS Platform)\n", version)

	case "pod":
		handlePodCommand(os.Args[2:])

	case "validate":
		handleValidateCommand(os.Args[2:])

	case "register":
		handleRegisterCommand(os.Args[2:])

	case "audit":
		handleAuditCommand(os.Args[2:])

	case "help", "--help", "-h":
		printUsage()

	default:
		fmt.Printf("Unknown command: %s\n\n", command)
		printUsage()
		os.Exit(1)
	}
}

func printUsage() {
	fmt.Println(`================================================================================
🛠️  aipods-cli - CLI oficial de AI Pods Enterprise SaaS Platform (v` + version + `)
================================================================================

USO:
  aipods-cli <command> [options]

COMANDOS DISPONIBLES:
  pod init --name=POD_NAME [--lang=go|python]   Inicializa un nuevo AI Pod
  validate --path=./path [--strict]             Ejecuta el pipeline de validación en 4 filtros
  register --id=POD_ID --endpoint=URL           Registra un Pod dinámico en el DynamicSmartRouter
  audit dossier --scope=global|tenant           Genera el expediente ISO 9001 / SOC 2 con firma OpenSSL
  audit verify --report=file.pdf --manifest=f   Verifica la integridad criptográfica del expediente
  version                                       Muestra la versión de aipods-cli

EJEMPLOS:
  aipods-cli pod init --name=POD_CUSTOM_FINANCE --lang=python
  aipods-cli validate --path=./pod_custom_finance --strict
  aipods-cli register --id=POD_CUSTOM_FINANCE --endpoint=http://localhost:9095 --keywords=tax,finance
  aipods-cli audit dossier --scope=global --out=./dist`)
}

func handlePodCommand(args []string) {
	if len(args) < 1 {
		fmt.Println("Usage: aipods-cli pod init --name=POD_NAME [--lang=go|python]")
		os.Exit(1)
	}

	subCmd := args[0]
	if subCmd != "init" {
		fmt.Printf("Unknown pod subcommand: %s\n", subCmd)
		os.Exit(1)
	}

	initFlags := flag.NewFlagSet("pod init", flag.ExitOnError)
	namePtr := initFlags.String("name", "", "Name of the AI Pod (e.g. POD_CUSTOM_FINANCE)")
	langPtr := initFlags.String("lang", "go", "Language implementation (go or python)")
	dirPtr := initFlags.String("dir", ".", "Target output directory")

	_ = initFlags.Parse(args[1:])

	if *namePtr == "" {
		fmt.Println("Error: --name parameter is required")
		os.Exit(1)
	}

	fmt.Printf("🚀 Inicializando AI Pod %s (%s)...\n", *namePtr, *langPtr)
	if err := scaffold.GeneratePodBoilerplate(*dirPtr, *namePtr, *langPtr); err != nil {
		fmt.Printf("❌ Error al generar el Pod: %v\n", err)
		os.Exit(1)
	}

	fmt.Printf("✅ AI Pod %s inicializado con éxito.\n", *namePtr)
}

func handleValidateCommand(args []string) {
	valFlags := flag.NewFlagSet("validate", flag.ExitOnError)
	pathPtr := valFlags.String("path", ".", "Path to the AI Pod directory")
	strictPtr := valFlags.Bool("strict", false, "Enforce strict zero-vulnerability checks")

	_ = valFlags.Parse(args)

	fmt.Printf("🔍 Ejecutando Pipeline de Validación en 4 Filtros para %s...\n\n", *pathPtr)

	report, err := validator.ValidatePodPipeline(*pathPtr, *strictPtr)
	if err != nil {
		fmt.Printf("❌ Error durante la validación: %v\n", err)
		os.Exit(1)
	}

	for _, f := range report.Filters {
		statusSymbol := "✅"
		if !f.Passed {
			statusSymbol = "❌"
		}
		fmt.Printf("%s [%s] %s (%v)\n", statusSymbol, f.FilterName, f.Details, f.Duration)
	}

	fmt.Println(strings.Repeat("-", 80))
	if report.AllPass {
		fmt.Printf("🟢 SUCCESS: AI Pod %s VALIDADO CON ÉXITO (%v).\n", report.PodID, report.Duration)
		fmt.Printf("   Listo para registro en caliente con: aipods-cli register --id=%s --endpoint=http://localhost:9095\n", report.PodID)
	} else {
		fmt.Printf("🔴 FAILED: La validación falló en uno o más filtros.\n")
		os.Exit(1)
	}
}

func handleRegisterCommand(args []string) {
	regFlags := flag.NewFlagSet("register", flag.ExitOnError)
	idPtr := regFlags.String("id", "", "ID of the AI Pod (e.g. POD_CUSTOM_FINANCE)")
	endpointPtr := regFlags.String("endpoint", "", "HTTP Sidecar Endpoint (e.g. http://localhost:9095)")
	keywordsPtr := regFlags.String("keywords", "", "Comma-separated keywords")
	serverPtr := regFlags.String("server", "http://localhost:8080", "AI Pods Core Engine server URL")

	_ = regFlags.Parse(args)

	if *idPtr == "" || *endpointPtr == "" {
		fmt.Println("Error: --id and --endpoint are required")
		os.Exit(1)
	}

	fmt.Printf("🔌 Registrando Pod Dinámico %s en el DynamicSmartRouter (%s)...\n", *idPtr, *endpointPtr)

	resp, err := register.RegisterDynamicPod(*serverPtr, *idPtr, *endpointPtr, *keywordsPtr)
	if err != nil {
		fmt.Printf("❌ Error al registrar el Pod: %v\n", err)
		os.Exit(1)
	}

	fmt.Printf("✅ %s: %s (Pod ID: %s)\n", resp.Status, resp.Message, resp.PodID)
}

func handleAuditCommand(args []string) {
	if len(args) < 1 {
		fmt.Println("Usage: aipods-cli audit [dossier|verify]")
		os.Exit(1)
	}

	subCmd := args[0]
	switch subCmd {
	case "dossier":
		auditFlags := flag.NewFlagSet("audit dossier", flag.ExitOnError)
		scopePtr := auditFlags.String("scope", "global", "Scope of audit: global or tenant")
		tenantPtr := auditFlags.String("tenant-id", "GLOBAL", "Tenant ID for tenant-level audit")
		outPtr := auditFlags.String("out", ".", "Output directory for generated dossier")

		_ = auditFlags.Parse(args[1:])

		fmt.Printf("📜 Compilando Dossier Normativo ISO 9001 / SOC 2 Type II (Scope: %s)...\n", *scopePtr)

		summary, err := audit.GenerateAuditDossier(*outPtr, *scopePtr, *tenantPtr)
		if err != nil {
			fmt.Printf("❌ Error al generar el dossier: %v\n", err)
			os.Exit(1)
		}

		fmt.Println(strings.Repeat("=", 80))
		fmt.Printf("✅ DOSSIER NORMATIVO GENERADO CON ÉXITO\n")
		fmt.Printf("   Archivo PDF:  %s\n", summary.ReportPath)
		fmt.Printf("   Manifiesto:   %s\n", summary.ManifestPath)
		fmt.Printf("   Hash SHA-256: %s\n", summary.SHA256Hash)
		fmt.Printf("   Specs SDD:    %d Específicaciones Auditadas\n", summary.SpecsCompiled)
		fmt.Printf("   Seguridad:    %d Vulnerabilidades gosec\n", summary.Vulnerabilities)
		fmt.Println(strings.Repeat("=", 80))

	case "verify":
		verifyFlags := flag.NewFlagSet("audit verify", flag.ExitOnError)
		reportPtr := verifyFlags.String("report", "", "Path to the dossier PDF report")
		manifestPtr := verifyFlags.String("manifest", "", "Path to the SHA256 manifest")

		_ = verifyFlags.Parse(args[1:])

		if *reportPtr == "" || *manifestPtr == "" {
			fmt.Println("Error: --report and --manifest are required")
			os.Exit(1)
		}

		valid, hashHex, err := audit.VerifyDossierManifest(*reportPtr, *manifestPtr)
		if err != nil {
			fmt.Printf("❌ Error al verificar el dossier: %v\n", err)
			os.Exit(1)
		}

		if valid {
			fmt.Printf("🟢 FIRMA DIGITAL Y MANIFIESTO VERIFICADOS CON ÉXITO.\n   Hash SHA-256: %s\n", hashHex)
		} else {
			fmt.Printf("🔴 FALLO DE VERIFICACIÓN: El hash no coincide o el reporte fue alterado.\n")
			os.Exit(1)
		}

	default:
		fmt.Printf("Unknown audit subcommand: %s\n", subCmd)
		os.Exit(1)
	}
}
