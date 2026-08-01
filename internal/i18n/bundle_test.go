package i18n_test

import (
	"testing"

	"github.com/martinllanos/only-ai-pods/internal/i18n"
)

func TestI18nFallbackCascade(t *testing.T) {
	translator := i18n.GetTranslator()

	tests := []struct {
		locale    string
		messageID string
		expected  string
	}{
		{"es_AR", "welcome_message", "Bienvenido a AI Pods Enterprise SaaS Platform"},
		{"es_CL", "welcome_message", "Bienvenido a AI Pods Enterprise SaaS Platform"},
		{"pt_BR", "welcome_message", "Bem-vindo ao AI Pods Enterprise SaaS Platform"},
		{"en_US", "welcome_message", "Welcome to AI Pods Enterprise SaaS Platform"},
		{"fr_FR", "welcome_message", "Welcome to AI Pods Enterprise SaaS Platform"}, // Fallback a 'en'
	}

	for _, tt := range tests {
		result := translator.Localize(tt.locale, tt.messageID)
		if result != tt.expected {
			t.Errorf("Localize(%s, %s) = %s; want %s", tt.locale, tt.messageID, result, tt.expected)
		}
	}
}
