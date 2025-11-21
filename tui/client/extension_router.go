package client

import (
	"context"
	"fmt"
	"path/filepath"

	"github.com/ron/tui_acp/tui/logger"
)

// ExtensionRouter handles custom extension methods that start with underscore.
// According to the ACP extensibility spec, method names starting with _ are reserved
// for custom extensions.
type ExtensionRouter struct {
	fs     *FileSystemAdapter
	logger logger.Logger
}

// NewExtensionRouter creates a new extension method router
func NewExtensionRouter(fs *FileSystemAdapter, log logger.Logger) *ExtensionRouter {
	if log == nil {
		log = logger.NewNoopLogger()
	}
	return &ExtensionRouter{
		fs:     fs,
		logger: log,
	}
}

// HandleExtensionMethod routes extension methods to their handlers
func (r *ExtensionRouter) HandleExtensionMethod(ctx context.Context, method string, params map[string]interface{}) (interface{}, error) {
	switch method {
	case "_fs/grep_search":
		return r.handleGrepSearch(ctx, params)
	default:
		return nil, fmt.Errorf("extension method not supported: %s", method)
	}
}

// handleGrepSearch handles the _fs/grep_search extension method
func (r *ExtensionRouter) handleGrepSearch(ctx context.Context, params map[string]interface{}) (interface{}, error) {
	r.logger.Info("HandleGrepSearch called with params: %+v", params)

	// Extract parameters
	pattern, _ := params["pattern"].(string)
	if pattern == "" {
		return nil, fmt.Errorf("pattern is required")
	}

	path, _ := params["path"].(string)
	if path == "" {
		path = "."
	}

	caseSensitive, _ := params["caseSensitive"].(bool)
	filePattern, _ := params["filePattern"].(string)

	// Resolve the path relative to working directory
	resolvedPath := r.fs.ResolvePath(path)

	r.logger.Debug("Grep search: pattern=%s, path=%s, caseSensitive=%v, filePattern=%s",
		pattern, resolvedPath, caseSensitive, filePattern)

	// Perform the grep search (recursive by default)
	results, err := r.fs.GrepSearch(ctx, pattern, []string{resolvedPath}, true, caseSensitive)
	if err != nil {
		r.logger.Error("GrepSearch failed: %v", err)
		return nil, err
	}

	// Convert results to the expected format and limit to 20 results
	return r.formatGrepResults(results, filePattern)
}

// formatGrepResults converts GrepResult slice to the expected response format
func (r *ExtensionRouter) formatGrepResults(results []GrepResult, filePattern string) (map[string]interface{}, error) {
	const maxResults = 20
	const maxLineLength = 200

	matches := make([]map[string]interface{}, 0, len(results))
	truncated := false
	hasFilePattern := filePattern != ""

	for _, result := range results {
		// Stop if we've reached the limit
		if len(matches) >= maxResults {
			truncated = true
			break
		}

		// Apply file pattern filter if specified
		if hasFilePattern {
			matched, err := filepath.Match(filePattern, filepath.Base(result.Path))
			if err != nil || !matched {
				continue
			}
		}

		// Truncate long lines to avoid huge JSON responses
		line := result.Line
		if len(line) > maxLineLength {
			line = line[:maxLineLength] + "..."
		}

		matches = append(matches, map[string]interface{}{
			"path":       result.Path,
			"lineNumber": result.LineNumber,
			"line":       line,
			"match":      result.Match,
		})
	}

	r.logger.Debug("Grep search found %d matches (truncated: %v)", len(matches), truncated)

	response := map[string]interface{}{
		"matches":   matches,
		"truncated": truncated,
	}

	if truncated {
		response["message"] = fmt.Sprintf("Results limited to %d matches. Refine your search for more specific results.", maxResults)
	}

	return response, nil
}
