package tools

import (
	"context"
	"fmt"
	"strconv"
	"strings"
	"math"
)

// CalculatorTool provides mathematical calculation capabilities
type CalculatorTool struct{}

// Name returns the tool name
func (t *CalculatorTool) Name() string {
	return "calculator"
}

// Description returns what the tool does
func (t *CalculatorTool) Description() string {
	return "Perform mathematical calculations. Supports basic operations (+, -, *, /), power (^), and common functions (sqrt, abs). Example: '2+3*4' or 'sqrt(16)' or '2^10'"
}

// Execute performs the calculation
func (t *CalculatorTool) Execute(ctx context.Context, expression string) (string, error) {
	expression = strings.TrimSpace(expression)
	if expression == "" {
		return "", fmt.Errorf("empty expression")
	}

	result, err := evaluate(expression)
	if err != nil {
		return "", fmt.Errorf("calculation error: %w", err)
	}

	return fmt.Sprintf("%.6f", result), nil
}

// evaluate is a simple expression evaluator
// Supports: +, -, *, /, ^, sqrt(), abs()
func evaluate(expr string) (float64, error) {
	expr = strings.ReplaceAll(expr, " ", "")

	// Handle sqrt function
	if strings.HasPrefix(expr, "sqrt(") && strings.HasSuffix(expr, ")") {
		inner := expr[5 : len(expr)-1]
		val, err := evaluate(inner)
		if err != nil {
			return 0, err
		}
		if val < 0 {
			return 0, fmt.Errorf("cannot take square root of negative number")
		}
		return math.Sqrt(val), nil
	}

	// Handle abs function
	if strings.HasPrefix(expr, "abs(") && strings.HasSuffix(expr, ")") {
		inner := expr[4 : len(expr)-1]
		val, err := evaluate(inner)
		if err != nil {
			return 0, err
		}
		return math.Abs(val), nil
	}

	// Handle parentheses (strip outer parentheses)
	if strings.HasPrefix(expr, "(") && strings.HasSuffix(expr, ")") {
		// Check if these are matching outer parentheses
		depth := 0
		for i, ch := range expr {
			if ch == '(' {
				depth++
			} else if ch == ')' {
				depth--
			}
			// If depth reaches 0 before the end, these aren't outer parentheses
			if depth == 0 && i < len(expr)-1 {
				break
			}
		}
		// If we made it through with depth 0, strip outer parentheses
		if depth == 0 {
			return evaluate(expr[1 : len(expr)-1])
		}
	}

	// Handle power operator (^) - skip operators inside parentheses
	depth := 0
	for i := len(expr) - 1; i >= 0; i-- {
		if expr[i] == ')' {
			depth++
		} else if expr[i] == '(' {
			depth--
		}
		if depth == 0 && expr[i] == '^' {
			left := expr[:i]
			right := expr[i+1:]
			leftVal, err := evaluate(left)
			if err != nil {
				return 0, err
			}
			rightVal, err := evaluate(right)
			if err != nil {
				return 0, err
			}
			return math.Pow(leftVal, rightVal), nil
		}
	}

	// Handle addition and subtraction (lowest precedence) - skip operators inside parentheses
	depth = 0
	for i := len(expr) - 1; i >= 0; i-- {
		if expr[i] == ')' {
			depth++
		} else if expr[i] == '(' {
			depth--
		}
		if depth == 0 && i > 0 {
			if expr[i] == '+' {
				left := expr[:i]
				right := expr[i+1:]
				leftVal, err := evaluate(left)
				if err != nil {
					return 0, err
				}
				rightVal, err := evaluate(right)
				if err != nil {
					return 0, err
				}
				return leftVal + rightVal, nil
			}
			if expr[i] == '-' {
				left := expr[:i]
				right := expr[i+1:]
				leftVal, err := evaluate(left)
				if err != nil {
					return 0, err
				}
				rightVal, err := evaluate(right)
				if err != nil {
					return 0, err
				}
				return leftVal - rightVal, nil
			}
		}
	}

	// Handle multiplication and division (higher precedence) - skip operators inside parentheses
	depth = 0
	for i := len(expr) - 1; i >= 0; i-- {
		if expr[i] == ')' {
			depth++
		} else if expr[i] == '(' {
			depth--
		}
		if depth == 0 {
			if expr[i] == '*' {
				left := expr[:i]
				right := expr[i+1:]
				leftVal, err := evaluate(left)
				if err != nil {
					return 0, err
				}
				rightVal, err := evaluate(right)
				if err != nil {
					return 0, err
				}
				return leftVal * rightVal, nil
			}
			if expr[i] == '/' {
				left := expr[:i]
				right := expr[i+1:]
				leftVal, err := evaluate(left)
				if err != nil {
					return 0, err
				}
				rightVal, err := evaluate(right)
				if err != nil {
					return 0, err
				}
				if rightVal == 0 {
					return 0, fmt.Errorf("division by zero")
				}
				return leftVal / rightVal, nil
			}
		}
	}

	// Try to parse as number
	val, err := strconv.ParseFloat(expr, 64)
	if err != nil {
		return 0, fmt.Errorf("invalid expression: %s", expr)
	}

	return val, nil
}
