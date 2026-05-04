package tui

import (
	"strings"
	"testing"
)

func setupStatusBar(t *testing.T) *StatusBar {
	t.Helper()
	return NewStatusBar(DefaultTheme())
}

func TestStatusBarSetModel(t *testing.T) {
	bar := setupStatusBar(t)
	bar.SetModel("test-model")

	rendered := bar.Render()
	if !strings.Contains(rendered, "test-model") {
		t.Errorf("Render() should contain model name: %q", rendered)
	}
}

func TestStatusBarSetTokenCount(t *testing.T) {
	bar := setupStatusBar(t)
	bar.SetTokenCount(1000)

	rendered := bar.Render()
	if !strings.Contains(rendered, "1000") {
		t.Errorf("Render() should contain token count: %q", rendered)
	}
}

func TestStatusBarSetMaxTokens(t *testing.T) {
	bar := setupStatusBar(t)
	bar.SetTokenCount(50000)
	bar.SetMaxTokens(100000)

	rendered := bar.Render()
	if !strings.Contains(rendered, "50000") {
		t.Errorf("Render() should contain current tokens: %q", rendered)
	}
	if !strings.Contains(rendered, "100000") {
		t.Errorf("Render() should contain max tokens: %q", rendered)
	}
	if !strings.Contains(rendered, "50%") {
		t.Errorf("Render() should contain percentage: %q", rendered)
	}
}

func TestStatusBarSetMaxTokensZero(t *testing.T) {
	bar := setupStatusBar(t)
	bar.SetMaxTokens(0)

	if bar.maxTokens != 131072 {
		t.Errorf("maxTokens = %d, should keep default when 0 passed", bar.maxTokens)
	}
}

func TestStatusBarSetState(t *testing.T) {
	bar := setupStatusBar(t)

	bar.SetState(StatusStreaming)
	rendered := bar.Render()
	if !strings.Contains(rendered, "streaming...") {
		t.Errorf("Render() should contain streaming state: %q", rendered)
	}

	bar.SetState(StatusCompacting)
	rendered = bar.Render()
	if !strings.Contains(rendered, "compacting...") {
		t.Errorf("Render() should contain compacting state: %q", rendered)
	}

	bar.SetState(StatusError)
	rendered = bar.Render()
	if !strings.Contains(rendered, "error") {
		t.Errorf("Render() should contain error state: %q", rendered)
	}

	bar.SetState(StatusProcessing)
	rendered = bar.Render()
	if !strings.Contains(rendered, "processing...") {
		t.Errorf("Render() should contain processing state: %q", rendered)
	}

	bar.SetState(StatusIdle)
	rendered = bar.Render()
	if !strings.Contains(rendered, "ready") {
		t.Errorf("Render() should contain ready state: %q", rendered)
	}
}

func TestStatusBarSetWidth(t *testing.T) {
	bar := setupStatusBar(t)
	bar.SetWidth(120)

	if bar.width != 120 {
		t.Errorf("width = %d, want 120", bar.width)
	}
}

func TestStatusBarRenderDefaultWidth(t *testing.T) {
	bar := setupStatusBar(t)

	rendered := bar.Render()
	if bar.width != 80 {
		t.Errorf("width = %d, want 80 (default)", bar.width)
	}
	if rendered == "" {
		t.Error("Render should not return empty string")
	}
}

func TestStatusBarTokenPercentage(t *testing.T) {
	bar := setupStatusBar(t)
	bar.SetTokenCount(25000)
	bar.SetMaxTokens(100000)

	rendered := bar.Render()
	if !strings.Contains(rendered, "25%") {
		t.Errorf("Render() should show 25%%: %q", rendered)
	}
}

func TestStatusBarZeroTokens(t *testing.T) {
	bar := setupStatusBar(t)
	bar.SetTokenCount(0)
	bar.SetMaxTokens(100000)

	rendered := bar.Render()
	if !strings.Contains(rendered, "0/100000") {
		t.Errorf("Render() should show 0/100000: %q", rendered)
	}
}

func TestStatusBarScrollHint(t *testing.T) {
	bar := setupStatusBar(t)
	bar.SetWidth(120)

	// No hint when zero.
	rendered := bar.Render()
	if strings.Contains(rendered, "↓") {
		t.Errorf("Render should not contain ↓ when scrollHint is 0: %q", rendered)
	}

	// Hint visible when set.
	bar.SetScrollHint(42)
	rendered = bar.Render()
	if !strings.Contains(rendered, "↓42") {
		t.Errorf("Render should contain ↓42: %q", rendered)
	}

	// Negative clamped to zero.
	bar.SetScrollHint(-5)
	if bar.scrollHint != 0 {
		t.Errorf("scrollHint = %d, want 0 after negative input", bar.scrollHint)
	}
}

func TestStatusBarStates(t *testing.T) {
	states := []StatusBarState{
		StatusIdle,
		StatusStreaming,
		StatusCompacting,
		StatusError,
		StatusProcessing,
	}

	expected := map[StatusBarState]bool{
		StatusIdle:       true,
		StatusStreaming:  true,
		StatusCompacting: true,
		StatusError:      true,
		StatusProcessing: true,
	}

	for _, state := range states {
		if !expected[state] {
			t.Errorf("unexpected status state: %q", state)
		}
	}
}
