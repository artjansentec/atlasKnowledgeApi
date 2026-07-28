package ai_test

import (
	"testing"

	"github.com/atlas/knowledge-api/internal/ai"
)

func TestNormalizeStage(t *testing.T) {
	if got := ai.NormalizeStage("GENERATING", "READING_CHUNKS"); got != "READING_CHUNKS" {
		t.Fatalf("prefer current_stage: %q", got)
	}
	if got := ai.NormalizeStage("extracting", ""); got != "EXTRACTING" {
		t.Fatalf("fallback status: %q", got)
	}
}

func TestStageLabel(t *testing.T) {
	if got := ai.StageLabel("READING_CHUNKS"); got != "Lendo em chunks" {
		t.Fatalf("got %q", got)
	}
	if got := ai.StageLabel("UNKNOWN_X"); got != "UNKNOWN_X" {
		t.Fatalf("unknown should pass through: %q", got)
	}
}

func TestIsTerminalStage(t *testing.T) {
	if !ai.IsTerminalStage("COMPLETED") || !ai.IsTerminalStage("failed") {
		t.Fatal("expected terminal")
	}
	if ai.IsTerminalStage("READING_CHUNKS") {
		t.Fatal("reading chunks is not terminal")
	}
}
