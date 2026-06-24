package context

import (
	"path/filepath"
)

func deduplicateContextFiles(files []string) []string {
	if len(files) == 0 {
		return files
	}

	seen := make(map[string]bool)
	var result []string

	for _, f := range files {
		absPath, err := filepath.Abs(f)
		if err != nil {
			absPath = f
		}
		absPath = filepath.Clean(absPath)

		if !seen[absPath] {
			seen[absPath] = true
			result = append(result, absPath)
		}
	}

	return result
}
