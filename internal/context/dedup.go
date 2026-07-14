package context

import (
	"hash/fnv"
	"os"
	"path/filepath"
	"strings"
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

	result = deduplicateByContentHash(result)

	if len(result) <= 10 {
		result = removeContainedFiles(result)
	}

	return result
}

func computeFileHash(filePath string, maxBytes int) (uint64, error) {
	f, err := os.Open(filePath)
	if err != nil {
		return 0, err
	}
	defer f.Close()

	buf := make([]byte, maxBytes)
	n, err := f.Read(buf)
	if err != nil {
		return 0, err
	}

	h := fnv.New64a()
	h.Write(buf[:n])
	return h.Sum64(), nil
}

func deduplicateByContentHash(files []string) []string {
	if len(files) <= 1 {
		return files
	}

	seen := make(map[uint64]bool)
	var result []string

	for _, f := range files {
		hash, err := computeFileHash(f, 1000)
		if err != nil {
			result = append(result, f)
			continue
		}
		if !seen[hash] {
			seen[hash] = true
			result = append(result, f)
		}
	}

	return result
}

func removeContainedFiles(files []string) []string {
	if len(files) <= 1 {
		return files
	}

	contents := make([]string, len(files))
	for i, f := range files {
		data, err := os.ReadFile(f)
		if err != nil {
			contents[i] = ""
			continue
		}
		contents[i] = string(data)
	}

	removed := make(map[int]bool)
	for i := 0; i < len(files); i++ {
		if removed[i] || contents[i] == "" {
			continue
		}
		for j := 0; j < len(files); j++ {
			if i == j || removed[j] || contents[j] == "" {
				continue
			}
			if len(contents[i]) < len(contents[j]) && strings.Contains(contents[j], contents[i]) {
				removed[i] = true
				break
			}
		}
	}

	var result []string
	for i, f := range files {
		if !removed[i] {
			result = append(result, f)
		}
	}
	return result
}
