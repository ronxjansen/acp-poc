package client

import (
	"bufio"
	"context"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"regexp"

	"github.com/ron/tui_acp/tui/logger"
)

// FileSystemAdapter handles file system operations with logging and path resolution
type FileSystemAdapter struct {
	cwd    string
	logger logger.Logger
}

// NewFileSystemAdapter creates a new FileSystemAdapter
func NewFileSystemAdapter(cwd string, log logger.Logger) *FileSystemAdapter {
	if log == nil {
		log = logger.NewNoopLogger()
	}
	return &FileSystemAdapter{
		cwd:    cwd,
		logger: log,
	}
}

// SetCwd updates the working directory
func (f *FileSystemAdapter) SetCwd(cwd string) {
	f.cwd = cwd
	f.logger.Debug("FileSystemAdapter cwd updated to: %s", cwd)
}

// ResolvePath resolves a path relative to the working directory
// If the path is already absolute, it returns it unchanged
func (f *FileSystemAdapter) ResolvePath(path string) string {
	if filepath.IsAbs(path) {
		return path
	}
	return filepath.Join(f.cwd, path)
}

// ResolveAndValidatePath resolves a path and validates it exists
func (f *FileSystemAdapter) ResolveAndValidatePath(path string) (string, error) {
	resolved := f.ResolvePath(path)
	if _, err := os.Stat(resolved); err != nil {
		return "", fmt.Errorf("path does not exist: %s", resolved)
	}
	return resolved, nil
}

// WriteTextFile writes content to a file, creating directories as needed
func (f *FileSystemAdapter) WriteTextFile(path string, content string) error {
	resolvedPath := f.ResolvePath(path)

	// Create parent directories if they don't exist
	dir := filepath.Dir(resolvedPath)
	if err := os.MkdirAll(dir, 0755); err != nil {
		f.logger.Error("Failed to create directory %s: %v", dir, err)
		return fmt.Errorf("failed to create directory: %w", err)
	}

	// Write the file content
	err := os.WriteFile(resolvedPath, []byte(content), 0644)
	f.logFileOperation("write", resolvedPath, len(content), err)
	if err != nil {
		return fmt.Errorf("failed to write file: %w", err)
	}

	return nil
}

// ReadTextFile reads content from a file
func (f *FileSystemAdapter) ReadTextFile(path string) (string, error) {
	resolvedPath := f.ResolvePath(path)

	content, err := os.ReadFile(resolvedPath)
	f.logFileOperation("read", resolvedPath, len(content), err)
	if err != nil {
		return "", fmt.Errorf("failed to read file: %w", err)
	}

	return string(content), nil
}

// GrepSearch searches for a pattern in files with context cancellation support
func (f *FileSystemAdapter) GrepSearch(ctx context.Context, pattern string, paths []string, recursive bool, caseSensitive bool) ([]GrepResult, error) {
	f.logger.Info("GrepSearch called with pattern: %s, paths: %v", pattern, paths)

	// Check for cancellation before starting
	if err := ctx.Err(); err != nil {
		return nil, err
	}

	// Compile the regex pattern
	var re *regexp.Regexp
	var err error
	if caseSensitive {
		re, err = regexp.Compile(pattern)
	} else {
		re, err = regexp.Compile("(?i)" + pattern)
	}
	if err != nil {
		f.logger.Error("Invalid regex pattern %s: %v", pattern, err)
		return nil, fmt.Errorf("invalid regex pattern: %w", err)
	}

	var results []GrepResult

	for _, path := range paths {
		// Check for cancellation between paths
		if err := ctx.Err(); err != nil {
			f.logger.Debug("GrepSearch cancelled after %d results", len(results))
			return results, err
		}

		info, err := os.Stat(path)
		if err != nil {
			f.logger.Error("Failed to stat path %s: %v", path, err)
			continue
		}

		if info.IsDir() {
			err := f.walkDirectory(ctx, path, recursive, false, func(filePath string, d fs.DirEntry) error {
				matches, _ := f.grepFile(filePath, re)
				results = append(results, matches...)
				return nil
			})
			if err != nil {
				// Context cancelled during walk
				return results, err
			}
		} else {
			matches, _ := f.grepFile(path, re)
			results = append(results, matches...)
		}
	}

	f.logger.Debug("GrepSearch found %d matches", len(results))
	return results, nil
}

// ListDirectories lists files and directories at the specified path
func (f *FileSystemAdapter) ListDirectories(ctx context.Context, path string, recursive bool) ([]DirectoryEntry, error) {
	f.logger.Info("ListDirectories called for path: %s, recursive: %v", path, recursive)

	info, err := os.Stat(path)
	if err != nil {
		f.logger.Error("Failed to stat path %s: %v", path, err)
		return nil, fmt.Errorf("failed to stat path: %w", err)
	}

	if !info.IsDir() {
		return nil, fmt.Errorf("path %s is not a directory", path)
	}

	var entries []DirectoryEntry

	err = f.walkDirectory(ctx, path, recursive, true, func(filePath string, d fs.DirEntry) error {
		info, err := d.Info()
		if err != nil {
			f.logger.Error("Failed to get info for %s: %v", filePath, err)
			return nil // Continue on error
		}

		entries = append(entries, DirectoryEntry{
			Path:  filePath,
			Name:  d.Name(),
			IsDir: d.IsDir(),
			Size:  info.Size(),
			Mode:  info.Mode(),
		})
		return nil
	})

	if err != nil && err != context.Canceled && err != context.DeadlineExceeded {
		return nil, err
	}

	f.logger.Debug("ListDirectories found %d entries", len(entries))
	return entries, nil
}

// walkDirectory is a unified directory walker that supports both recursive and non-recursive modes.
// It handles context cancellation and can include or exclude directories based on includeDirs.
func (f *FileSystemAdapter) walkDirectory(ctx context.Context, dirPath string, recursive bool, includeDirs bool, callback func(filePath string, d fs.DirEntry) error) error {
	if recursive {
		return filepath.WalkDir(dirPath, func(filePath string, d fs.DirEntry, err error) error {
			// Check for cancellation
			if ctxErr := ctx.Err(); ctxErr != nil {
				return ctxErr
			}

			if err != nil {
				f.logger.Error("Error walking path %s: %v", filePath, err)
				return nil // Continue on error
			}

			// Skip the root directory itself
			if filePath == dirPath {
				return nil
			}

			// Skip directories if not including them (for file-only operations like grep)
			if d.IsDir() && !includeDirs {
				return nil
			}

			return callback(filePath, d)
		})
	}

	// Non-recursive: just read the directory entries
	dirEntries, err := os.ReadDir(dirPath)
	if err != nil {
		f.logger.Error("Failed to read directory %s: %v", dirPath, err)
		return fmt.Errorf("failed to read directory: %w", err)
	}

	for _, entry := range dirEntries {
		// Check for cancellation
		if ctxErr := ctx.Err(); ctxErr != nil {
			return ctxErr
		}

		// Skip directories if not including them
		if entry.IsDir() && !includeDirs {
			continue
		}

		fullPath := filepath.Join(dirPath, entry.Name())
		if err := callback(fullPath, entry); err != nil {
			return err
		}
	}

	return nil
}

// grepFile searches for pattern matches in a single file
func (f *FileSystemAdapter) grepFile(filePath string, re *regexp.Regexp) ([]GrepResult, error) {
	file, err := os.Open(filePath)
	if err != nil {
		return nil, err
	}
	defer file.Close()

	if !isTextFileFromHandle(file) {
		return nil, nil
	}

	if _, err := file.Seek(0, 0); err != nil {
		return nil, err
	}

	var results []GrepResult
	scanner := bufio.NewScanner(file)
	lineNumber := 0

	for scanner.Scan() {
		lineNumber++
		line := scanner.Text()

		if match := re.FindString(line); match != "" {
			results = append(results, GrepResult{
				Path:       filePath,
				LineNumber: lineNumber,
				Line:       line,
				Match:      match,
			})
		}
	}

	if err := scanner.Err(); err != nil {
		return results, err
	}

	return results, nil
}

// logFileOperation logs file operations consistently
func (f *FileSystemAdapter) logFileOperation(op string, path string, size int, err error) {
	if err != nil {
		f.logger.Error("Failed to %s file %s: %v", op, path, err)
	} else {
		f.logger.Debug("Successfully %s %d bytes %s %s", op, size, getPreposition(op), path)
	}
}

// getPreposition returns the appropriate preposition for file operation logging
func getPreposition(op string) string {
	switch op {
	case "read":
		return "from"
	case "write":
		return "to"
	default:
		return "at"
	}
}

// isTextFileFromHandle checks if an already-opened file is likely a text file
// by reading the first 512 bytes and checking for null bytes or excessive non-printable characters.
// The file position will be advanced by up to 512 bytes after this call.
func isTextFileFromHandle(file *os.File) bool {
	// Read first 512 bytes
	buf := make([]byte, 512)
	n, err := file.Read(buf)
	if err != nil && n == 0 {
		return false
	}
	buf = buf[:n]

	// Single pass: check for null bytes and count non-printable characters
	var nonPrintable int
	for _, b := range buf {
		// Null byte is a strong indicator of binary file - return immediately
		if b == 0 {
			return false
		}
		// Count non-printable (excluding common whitespace: tab=9, newline=10, carriage return=13)
		if (b < 32 && b != 9 && b != 10 && b != 13) || (b > 126 && b < 128) {
			nonPrintable++
		}
	}

	// If more than 30% non-printable, consider it binary
	threshold := len(buf) * 30 / 100
	return nonPrintable < threshold
}
