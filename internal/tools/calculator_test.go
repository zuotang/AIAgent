package tools

import (
	"context"
	"testing"
)

func TestCalculatorTool(t *testing.T) {
	calc := &CalculatorTool{}
	ctx := context.Background()

	tests := []struct {
		name       string
		expression string
		want       string
		wantErr    bool
	}{
		{"simple addition", "2+3", "5.000000", false},
		{"simple subtraction", "10-3", "7.000000", false},
		{"simple multiplication", "4*5", "20.000000", false},
		{"simple division", "20/4", "5.000000", false},
		{"order of operations", "2+3*4", "14.000000", false},
		{"parentheses", "(2+3)*4", "20.000000", false},
		{"power", "2^10", "1024.000000", false},
		{"sqrt", "sqrt(16)", "4.000000", false},
		{"abs positive", "abs(5)", "5.000000", false},
		{"abs negative", "abs(-5)", "5.000000", false},
		{"complex", "2+3*4-10/2", "9.000000", false},
		{"division by zero", "10/0", "", true},
		{"empty expression", "", "", true},
		{"invalid expression", "abc", "", true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := calc.Execute(ctx, tt.expression)
			if (err != nil) != tt.wantErr {
				t.Errorf("Execute() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if !tt.wantErr && got != tt.want {
				t.Errorf("Execute() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestCalculatorToolMetadata(t *testing.T) {
	calc := &CalculatorTool{}

	if calc.Name() != "calculator" {
		t.Errorf("Name() = %v, want calculator", calc.Name())
	}

	desc := calc.Description()
	if desc == "" {
		t.Error("Description() should not be empty")
	}
}
