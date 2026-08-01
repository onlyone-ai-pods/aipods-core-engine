package i18n

import (
	"encoding/json"
	"fmt"
	"strings"
	"sync"

	"github.com/nicksnyder/go-i18n/v2/i18n"
	"golang.org/x/text/language"
)

// Translator administra la carga de diccionarios y resolución de fallback (SPEC-CORE-46 / Issue #25)
type Translator struct {
	mu     sync.RWMutex
	bundle *i18n.Bundle
}

var globalTranslator *Translator
var once sync.Once

// GetTranslator retorna el Translate singleton
func GetTranslator() *Translator {
	once.Do(func() {
		bundle := i18n.NewBundle(language.English)
		bundle.RegisterUnmarshalFunc("json", json.Unmarshal)

		// Cargar traducciones predeterminadas de emergencia
		globalTranslator = &Translator{
			bundle: bundle,
		}
		globalTranslator.loadDefaultTranslations()
	})
	return globalTranslator
}

func (t *Translator) loadDefaultTranslations() {
	// Idioma Español Neutro (es)
	_ = t.bundle.AddMessages(language.Spanish, &i18n.Message{
		ID:    "welcome_message",
		Other: "Bienvenido a AI Pods Enterprise SaaS Platform",
	}, &i18n.Message{
		ID:    "access_denied",
		Other: "Acceso denegado: Credenciales inválidas o desafío 2FA requerido",
	})

	// Idioma Portugués (pt)
	_ = t.bundle.AddMessages(language.Portuguese, &i18n.Message{
		ID:    "welcome_message",
		Other: "Bem-vindo ao AI Pods Enterprise SaaS Platform",
	}, &i18n.Message{
		ID:    "access_denied",
		Other: "Acesso negado: Credenciais inválidas ou desafio 2FA necessário",
	})

	// Idioma Inglés (en - Default)
	_ = t.bundle.AddMessages(language.English, &i18n.Message{
		ID:    "welcome_message",
		Other: "Welcome to AI Pods Enterprise SaaS Platform",
	}, &i18n.Message{
		ID:    "access_denied",
		Other: "Access denied: Invalid credentials or 2FA challenge required",
	})
}

// Localize traduce un ID utilizando la Cascada de Fallback (es_XX -> es -> en)
func (t *Translator) Localize(locale string, messageID string) string {
	t.mu.RLock()
	defer t.mu.RUnlock()

	// Normalizar locale (ej. es-AR -> es_AR -> es)
	targetLang := strings.ReplaceAll(locale, "-", "_")
	if targetLang == "" {
		targetLang = "en"
	}

	localizer := i18n.NewLocalizer(t.bundle, targetLang, strings.Split(targetLang, "_")[0], "en")
	translated, err := localizer.Localize(&i18n.LocalizeConfig{
		MessageID: messageID,
	})

	if err != nil || translated == "" {
		// Fallback manual de emergencia
		return fmt.Sprintf("[%s]", messageID)
	}

	return translated
}
