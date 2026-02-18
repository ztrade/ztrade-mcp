package tools

import (
	"fmt"
	"strings"
	"testing"
)

func TestCaptureStdoutLines(t *testing.T) {
	captured, err := captureStdoutLines(func() error {
		fmt.Println("line1")
		fmt.Println("line2")
		return nil
	})
	if err != nil {
		t.Fatalf("capture returned error: %v", err)
	}
	if captured.Truncated {
		t.Fatalf("capture should not be truncated")
	}
	if len(captured.Lines) != 2 {
		t.Fatalf("expected 2 lines, got %d", len(captured.Lines))
	}
	if strings.TrimSpace(captured.Lines[0]) != "line1" {
		t.Fatalf("unexpected first line: %q", captured.Lines[0])
	}
	if strings.TrimSpace(captured.Lines[1]) != "line2" {
		t.Fatalf("unexpected second line: %q", captured.Lines[1])
	}
}
