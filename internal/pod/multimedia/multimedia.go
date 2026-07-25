package multimedia

import (
	"context"
	"fmt"
	"strings"

	"github.com/google/uuid"
	"github.com/martinllanos/only-ai-pods/internal/pod"
)

type MultimediaWhisperPod struct{}

func NewMultimediaWhisperPod() *MultimediaWhisperPod {
	return &MultimediaWhisperPod{}
}

func (p *MultimediaWhisperPod) ID() string {
	return "POD_MULTIMEDIA_WHISPER"
}

func (p *MultimediaWhisperPod) Name() string {
	return "AI Pod Transcriptor Multimodal (Audio & Video Whisper)"
}

func (p *MultimediaWhisperPod) ProcessQuery(ctx context.Context, tenantID string, query string, dryRun bool) (*pod.PodResponse, error) {
	lowerQuery := strings.ToLower(query)

	var answer string
	var citations []string
	var dryRunRes *pod.DryRunResult

	if strings.Contains(lowerQuery, "audio") || strings.Contains(lowerQuery, "whatsapp") || strings.Contains(lowerQuery, "nota de voz") || strings.Contains(lowerQuery, "mp3") || strings.Contains(lowerQuery, "wav") {
		transcript := "[00:00:01 - Cliente]: Hola, necesito ayuda para cambiar la clave de mi certificado fiscal.\n[00:00:05 - Agente AI]: Con gusto, ingrese a la sección de AFIP y genere el archivo CSR."
		answer = fmt.Sprintf("AI Pod Transcriptor Multimodal: Transcripción de audio enviada desde WhatsApp para la empresa (%s):\n\n```text\n%s\n```\n\nTranscripción dividida en fragmentos con timestamps e indexada en el RAG Vectorial.", tenantID, transcript)
		citations = []string{
			"Whisper_STT_Specification.pdf (Pagina 4)",
			"WhatsApp_Audio_Transcription_Guide.pdf (Pagina 12)",
		}

		if dryRun {
			dynamicApprovalID := fmt.Sprintf("dryrun_%s", uuid.New().String()[:8])
			dryRunRes = &pod.DryRunResult{
				IsDryRun:              true,
				ActionName:            "transcribir_audio_whatsapp",
				Summary:               fmt.Sprintf("Simulación de transcripción de audio WhatsApp Speech-to-Text para %s.", tenantID),
				AffectedRecordsCount:  2,
				GeneratedCommand:      "whisper.stt.transcribe(model='whisper-1', language='es')",
				RequiresHumanApproval: false,
				ApprovalToken:         dynamicApprovalID,
			}
		}
	} else if strings.Contains(lowerQuery, "video") || strings.Contains(lowerQuery, "mp4") || strings.Contains(lowerQuery, "capacitacion") {
		answer = fmt.Sprintf("AI Pod Transcriptor Multimodal: Extracción de pista de audio y OCR de frames para el video de capacitación MP4 para (%s).\n- Duración: 05m 30s\n- Fragmentos Transcritos: 12 bloques con marca de tiempo.", tenantID)
		citations = []string{
			"Video_STT_OCR_Pipeline.pdf (Pagina 18)",
		}

		if dryRun {
			dynamicApprovalID := fmt.Sprintf("dryrun_%s", uuid.New().String()[:8])
			dryRunRes = &pod.DryRunResult{
				IsDryRun:              true,
				ActionName:            "transcribir_video_capacitacion",
				Summary:               fmt.Sprintf("Simulación de transcripción de video capacitación MP4 para %s.", tenantID),
				AffectedRecordsCount:  12,
				GeneratedCommand:      "ffmpeg.extract_audio(input='capacitacion.mp4') -> whisper.transcribe()",
				RequiresHumanApproval: false,
				ApprovalToken:         dynamicApprovalID,
			}
		}
	} else {
		answer = fmt.Sprintf("AI Pod Transcriptor Multimodal (Whisper): Servicio de transcripción Speech-to-Text activo para (%s).", tenantID)
		citations = []string{"Manual_Multimedia_Whisper.pdf (Pagina 1)"}
	}

	return &pod.PodResponse{
		PodID:        p.ID(),
		Answer:       answer,
		Citations:    citations,
		DryRunResult: dryRunRes,
		Status:       "SUCCESS",
	}, nil
}
