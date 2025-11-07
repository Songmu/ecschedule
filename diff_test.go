package ecschedule

import (
	"fmt"
	"strings"
	"testing"

	"github.com/fatih/color"
)

func TestFormatDiff(t *testing.T) {
	tests := []struct {
		name     string
		ruleName string
		from     string
		to       string
		format   diffFormat
		want     string // Expected substring in output
	}{
		{
			name:     "no difference",
			ruleName: "test-rule",
			from:     "name: test\nvalue: 1\n",
			to:       "name: test\nvalue: 1\n",
			format:   diffFormatPrettyColored,
			want:     "",
		},
		{
			name:     "empty from (new rule)",
			ruleName: "test-rule",
			from:     "",
			to:       "name: test\nvalue: 1\n",
			format:   diffFormatUnified,
			want:     "+name: test",
		},
		{
			name:     "empty to (deleted rule)",
			ruleName: "test-rule",
			from:     "name: test\nvalue: 1\n",
			to:       "",
			format:   diffFormatUnified,
			want:     "-name: test",
		},
		{
			name:     "unified format: header",
			ruleName: "test-rule",
			from:     "name: test\nvalue: 1\n",
			to:       "name: test\nvalue: 2\n",
			format:   diffFormatUnified,
			want:     "--- a/test-rule",
		},
		{
			name:     "unified format: change line",
			ruleName: "test-rule",
			from:     "name: test\nvalue: 1\n",
			to:       "name: test\nvalue: 2\n",
			format:   diffFormatUnified,
			want:     "-value: 1",
		},
		{
			name:     "pretty format: ANSI color codes",
			ruleName: "test-rule",
			from:     "name: test\nvalue: 1\n",
			to:       "name: test\nvalue: 2\n",
			format:   diffFormatPrettyColored,
			want:     "\x1b[",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := formatDiff(tt.ruleName, tt.from, tt.to, tt.format)
			if tt.want == "" {
				if got != "" {
					t.Errorf("formatDiff() = %q, want empty string", got)
				}
			} else {
				if !strings.Contains(got, tt.want) {
					t.Errorf("formatDiff() output does not contain %q\ngot:\n%s", tt.want, got)
				}
			}
		})
	}
}

func TestColorControl(t *testing.T) {
	from := "name: test\nvalue: 1\n"
	to := "name: test\nvalue: 2\n"

	// Pretty format is always colored
	prettyOutput := formatDiff("test", from, to, diffFormatPrettyColored)
	if !strings.Contains(prettyOutput, "\x1b[") {
		t.Error("Pretty format should always contain ANSI escape codes")
	}

	// Unified format with color.NoColor=false is colored
	color.NoColor = false
	unifiedColored := formatDiff("test", from, to, diffFormatUnified)
	if !strings.Contains(unifiedColored, "\x1b[") {
		t.Error("Unified format should contain ANSI escape codes when color.NoColor=false")
	}

	// Unified format with color.NoColor=true is plain
	color.NoColor = true
	unifiedPlain := formatDiff("test", from, to, diffFormatUnified)
	if strings.Contains(unifiedPlain, "\x1b[") {
		t.Error("Unified format should not contain ANSI escape codes when color.NoColor=true")
	}

	// Reset after test
	color.NoColor = false
}

func TestFormatDiffMultiByte(t *testing.T) {
	from := "name: テスト\ndescription: 日本語\n"
	to := "name: テスト\ndescription: 日本語説明\n"

	// Test multi-byte character handling in Unified format
	color.NoColor = true
	output := formatDiff("test", from, to, diffFormatUnified)
	if !strings.Contains(output, "日本語") {
		t.Error("Unified format should handle multi-byte characters")
	}
	if !strings.Contains(output, "-description: 日本語") {
		t.Error("Should show deleted line with multi-byte characters")
	}
	if !strings.Contains(output, "+description: 日本語説明") {
		t.Error("Should show added line with multi-byte characters")
	}

	// Reset after test
	color.NoColor = false
}

func BenchmarkFormatDiff(b *testing.B) {
	// Large YAML (100 lines)
	var fromLines, toLines []string
	for i := 0; i < 100; i++ {
		fromLines = append(fromLines, fmt.Sprintf("field%d: value%d", i, i))
		if i == 50 {
			toLines = append(toLines, fmt.Sprintf("field%d: changed%d", i, i))
		} else {
			toLines = append(toLines, fmt.Sprintf("field%d: value%d", i, i))
		}
	}
	from := strings.Join(fromLines, "\n")
	to := strings.Join(toLines, "\n")

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		formatDiff("benchmark-rule", from, to, diffFormatUnified)
	}
}
