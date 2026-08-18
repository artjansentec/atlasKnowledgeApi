package ai

import "strings"

// Estágios do job Mnemos (GET /v1/jobs/:id → status / current_stage).
// Mantidos alinhados a github.com/mnemos/mnemos/internal/models.
const (
	StageReceived             = "RECEIVED"
	StageValidating           = "VALIDATING"
	StageExtracting           = "EXTRACTING"
	StageOCR                  = "OCR"
	StageTranscribing         = "TRANSCRIBING"
	StageBuildingContext      = "BUILDING_CONTEXT"
	StageReadingChunks        = "READING_CHUNKS"
	StageBuildingPrompt       = "BUILDING_PROMPT"
	StageGenerating           = "GENERATING"
	StageAnalyzingSpecialists = "ANALYZING_SPECIALISTS"
	StageCoverage             = "COVERAGE"
	StageSynthesis            = "SYNTHESIS"
	StageEnriching            = "ENRICHING"
	StagePolishing            = "POLISHING"
	StageFinalValidation      = "FINAL_VALIDATION"
	StageValidatingResponse   = "VALIDATING_RESPONSE"
	StageCompleted            = "COMPLETED"
	StageFailed               = "FAILED"
)

var stageLabels = map[string]string{
	StageReceived:             "Job criado",
	StageValidating:           "Checando arquivos",
	StageExtracting:           "Lendo arquivos",
	StageOCR:                  "OCR (reservado)",
	StageTranscribing:         "Transcrição (reservado)",
	StageBuildingContext:      "Consolidando texto",
	StageReadingChunks:        "Lendo em chunks",
	StageBuildingPrompt:       "Montando prompts",
	StageGenerating:           "Gerando documentação",
	StageAnalyzingSpecialists: "Analisando especialistas",
	StageCoverage:             "Validando cobertura",
	StageSynthesis:            "Sintetizando documentação",
	StageEnriching:            "Enriquecendo",
	StagePolishing:            "Polindo",
	StageFinalValidation:      "Validação final",
	StageValidatingResponse:   "Validando / sincronizando",
	StageCompleted:            "Concluído",
	StageFailed:               "Falhou",
}

// NormalizeStage returns the canonical Mnemos stage code (uppercase), or empty.
func NormalizeStage(status, currentStage string) string {
	stage := strings.TrimSpace(currentStage)
	if stage == "" {
		stage = strings.TrimSpace(status)
	}
	return strings.ToUpper(stage)
}

// StageLabel returns a short Portuguese label for a Mnemos stage code.
// Unknown codes are returned unchanged so the front can still display them.
func StageLabel(stage string) string {
	code := strings.ToUpper(strings.TrimSpace(stage))
	if code == "" {
		return ""
	}
	if label, ok := stageLabels[code]; ok {
		return label
	}
	return stage
}

// IsTerminalStage reports Mnemos job completion.
func IsTerminalStage(stage string) bool {
	switch strings.ToUpper(strings.TrimSpace(stage)) {
	case StageCompleted, StageFailed:
		return true
	default:
		return false
	}
}
