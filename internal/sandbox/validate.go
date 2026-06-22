package sandbox

import (
	"fmt"
	"strings"
)

type ValidationError struct {
	ToolID string
	Param  string
	Issue  string
}

func (e ValidationError) Error() string {
	return fmt.Sprintf("tool %q param %q: %s", e.ToolID, e.Param, e.Issue)
}

func ValidateToolArgs(toolID string, args map[string]any, required []string, properties map[string]string) []ValidationError {
	var errs []ValidationError

	for _, req := range required {
		val, exists := args[req]
		if !exists || val == nil {
			errs = append(errs, ValidationError{ToolID: toolID, Param: req, Issue: "required parameter missing"})
			continue
		}
		if str, ok := val.(string); ok && strings.TrimSpace(str) == "" {
			errs = append(errs, ValidationError{ToolID: toolID, Param: req, Issue: "required parameter is empty"})
		}
	}

	for param, expectedType := range properties {
		val, exists := args[param]
		if !exists || val == nil {
			continue
		}

		switch expectedType {
		case "string":
			if _, ok := val.(string); !ok {
				errs = append(errs, ValidationError{ToolID: toolID, Param: param, Issue: fmt.Sprintf("expected string, got %T", val)})
			}
		case "number":
			switch val.(type) {
			case float64, float32, int, int64, int32:
			default:
				errs = append(errs, ValidationError{ToolID: toolID, Param: param, Issue: fmt.Sprintf("expected number, got %T", val)})
			}
		case "boolean":
			if _, ok := val.(bool); !ok {
				errs = append(errs, ValidationError{ToolID: toolID, Param: param, Issue: fmt.Sprintf("expected boolean, got %T", val)})
			}
		}
	}

	if toolID == "write_file" || toolID == "edit_file" {
		if path, ok := args["path"].(string); ok {
			if strings.Contains(path, "..") {
				errs = append(errs, ValidationError{ToolID: toolID, Param: "path", Issue: "path traversal detected"})
			}
		}
	}

	if toolID == "execute_command" {
		if cmd, ok := args["command"].(string); ok {
			if len(cmd) > 10000 {
				errs = append(errs, ValidationError{ToolID: toolID, Param: "command", Issue: "command too long (max 10000 chars)"})
			}
		}
	}

	if toolID == "http_request" {
		if url, ok := args["url"].(string); ok {
			if len(url) > 2048 {
				errs = append(errs, ValidationError{ToolID: toolID, Param: "url", Issue: "URL too long (max 2048 chars)"})
			}
		}
	}

	return errs
}
