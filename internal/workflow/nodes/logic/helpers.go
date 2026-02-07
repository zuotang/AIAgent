package logic

import (
	"fmt"
	"strconv"
	"strings"
)

// parseCases 解析 cases 参数，支持 []any 和 []string
func parseCases(raw any) ([]string, error) {
	if raw == nil {
		return nil, nil
	}

	switch v := raw.(type) {
	case []any:
		cases := make([]string, 0, len(v))
		for _, item := range v {
			s, ok := item.(string)
			if !ok {
				return nil, fmt.Errorf("cases items must be strings, got %T", item)
			}
			cases = append(cases, s)
		}
		return cases, nil
	case []string:
		return v, nil
	default:
		return nil, fmt.Errorf("cases must be an array, got %T", raw)
	}
}

// matchValue 根据模式匹配值
func matchValue(value, pattern, mode string) bool {
	switch mode {
	case "exact":
		return value == pattern
	case "contains":
		return strings.Contains(value, pattern)
	case "prefix":
		return strings.HasPrefix(value, pattern)
	case "suffix":
		return strings.HasSuffix(value, pattern)
	case "iexact":
		return strings.EqualFold(value, pattern)
	case "icontains":
		return strings.Contains(strings.ToLower(value), strings.ToLower(pattern))
	default:
		return value == pattern
	}
}

// evaluate 执行条件判断
func evaluate(value, operator, compare string) bool {
	switch operator {
	case "eq":
		return value == compare
	case "neq":
		return value != compare
	case "contains":
		return strings.Contains(value, compare)
	case "not_contains":
		return !strings.Contains(value, compare)
	case "prefix":
		return strings.HasPrefix(value, compare)
	case "suffix":
		return strings.HasSuffix(value, compare)
	case "empty":
		return strings.TrimSpace(value) == ""
	case "not_empty":
		return strings.TrimSpace(value) != ""
	case "gt", "lt", "gte", "lte":
		return compareNumeric(value, operator, compare)
	default:
		return value == compare
	}
}

// compareNumeric 数值比较
func compareNumeric(value, operator, compare string) bool {
	v, err1 := strconv.ParseFloat(value, 64)
	c, err2 := strconv.ParseFloat(compare, 64)
	if err1 != nil || err2 != nil {
		// 无法解析为数字，回退到字符串比较
		switch operator {
		case "gt":
			return value > compare
		case "lt":
			return value < compare
		case "gte":
			return value >= compare
		case "lte":
			return value <= compare
		}
		return false
	}

	switch operator {
	case "gt":
		return v > c
	case "lt":
		return v < c
	case "gte":
		return v >= c
	case "lte":
		return v <= c
	}
	return false
}
