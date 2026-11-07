package security

import (
	"bytes"
	"errors"
	"fmt"
	"regexp"
	"strings"
)

var (
	ErrFileTooLarge            = errors.New("file exceeds maximum allowed size limit of 10MB")
	ErrInvalidMagicBytes       = errors.New("file magic bytes mismatch: potential polyglot file attack detected")
	ErrMaliciousPDFObject      = errors.New("security threat: PDF contains malicious embedded executable objects (/JavaScript or /Launch)")
	ErrPromptInjectionDetected = errors.New("security threat: indirect RAG prompt injection pattern detected and neutralized")
)

// maliciousPDFName matches PDF names that run code, launch programs or carry files.
var maliciousPDFName = regexp.MustCompile(`/(JavaScript|JS|Launch|EmbeddedFiles?|SubmitForm|ImportData|RichMedia)(?:[\s/<>\[\]()%{}]|$)`)

const MaxFileSize = 10 * 1024 * 1024 // 10MB Limit

type SanitizationResult struct {
	IsSafe             bool     `json:"is_safe"`
	SanitizedContent   string   `json:"sanitized_content"`
	DetectedThreats    []string `json:"detected_threats"`
	OriginalByteLength int      `json:"original_byte_length"`
}

type FileSanitizer struct {
	injectionPatterns []*regexp.Regexp
}

func NewFileSanitizer() *FileSanitizer {
	return &FileSanitizer{
		injectionPatterns: []*regexp.Regexp{
			regexp.MustCompile(`(?i)ignore\s+(all\s+)?previous\s+instructions`),
			regexp.MustCompile(`(?i)system\s+prompt\s+override`),
			regexp.MustCompile(`(?i)output\s+(all\s+)?api\s+keys`),
			regexp.MustCompile(`(?i)reveal\s+(jwt\s+)?secret`),
		},
	}
}

// ValidatePDFMagicBytes verifies true PDF header (%PDF-)
func (s *FileSanitizer) ValidatePDFMagicBytes(data []byte) error {
	if len(data) > MaxFileSize {
		return ErrFileTooLarge
	}
	if len(data) < 5 || !bytes.HasPrefix(data, []byte("%PDF-")) {
		return ErrInvalidMagicBytes
	}

	// Scan for executable PDF objects. Names are matched as whole PDF name tokens,
	// so "/AAJ..." inside base64 data or "/JSmith" does not count. /OpenAction and
	// /AA alone are not blocked: ARCA's official PDFs use them to set the opening
	// view, and they can only run code through the action types listed here.
	if m := maliciousPDFName.FindSubmatch(data); m != nil {
		return fmt.Errorf("%w: object /%s found", ErrMaliciousPDFObject, m[1])
	}

	return nil
}

// SanitizeTextContent removes zero-width characters and neutralizes indirect prompt injection
func (s *FileSanitizer) SanitizeTextContent(content string) *SanitizationResult {
	threats := make([]string, 0)
	cleanContent := content

	// 1. Remove Zero-Width Unicode characters used for hidden watermarking
	zeroWidthReplacer := strings.NewReplacer(
		"\u200B", "",
		"\u200C", "",
		"\u200D", "",
		"\uFEFF", "",
	)
	cleanContent = zeroWidthReplacer.Replace(cleanContent)

	// 2. Scan and neutralize RAG prompt injection patterns
	for _, pattern := range s.injectionPatterns {
		if pattern.MatchString(cleanContent) {
			threats = append(threats, fmt.Sprintf("Prompt Injection Pattern: %s", pattern.String()))
			cleanContent = pattern.ReplaceAllString(cleanContent, "[PROMPT_INJECTION_BLOCKED]")
		}
	}

	return &SanitizationResult{
		IsSafe:             len(threats) == 0,
		SanitizedContent:   cleanContent,
		DetectedThreats:    threats,
		OriginalByteLength: len(content),
	}
}
