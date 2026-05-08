package service

import "testing"

func TestMatchPanelLogLevel(t *testing.T) {
	tests := []struct {
		name  string
		line  string
		level string
		want  bool
	}{
		{name: "info matches info threshold", line: "[INFO] started", level: "info", want: true},
		{name: "warn matches info threshold", line: "[WARN] warn text", level: "info", want: true},
		{name: "debug filtered by info threshold", line: "[DEBUG] debug text", level: "info", want: false},
		{name: "error matches warn threshold", line: "[ERROR] boom", level: "warn", want: true},
		{name: "warn filtered by error threshold", line: "[WARN] not error", level: "error", want: false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := MatchPanelLogLevel(tt.line, tt.level); got != tt.want {
				t.Fatalf("expected %v, got %v", tt.want, got)
			}
		})
	}
}
