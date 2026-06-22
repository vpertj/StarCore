package rag

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"math"
	"net/http"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"sync"
	"time"

	"StarCore/internal/provider"
)

type Document struct {
	ID        string            `json:"id"`
	Content   string            `json:"content"`
	Metadata  map[string]string `json:"metadata"`
	Embedding []float32         `json:"embedding,omitempty"`
}

type SearchResult struct {
	Document  *Document `json:"document"`
	Score     float64   `json:"score"`
	ChunkText string    `json:"chunkText"`
}

type Index struct {
	documents    []*Document
	chunkMap     map[string][]*Document
	embeddings   map[string][]float32
	fileModTimes map[string]time.Time
	mu           sync.RWMutex
	projectDir   string
	updatedAt    time.Time
}

type Service struct {
	providerMgr *provider.Manager
	indices     map[string]*Index
	mu          sync.RWMutex
	chunkSize   int
	overlap     int
}

func NewService(providerMgr *provider.Manager) *Service {
	return &Service{
		providerMgr: providerMgr,
		indices:     make(map[string]*Index),
		chunkSize:   512,
		overlap:     64,
	}
}

func (s *Service) IndexProject(ctx context.Context, projectDir string) error {
	s.mu.Lock()
	idx, exists := s.indices[projectDir]
	if !exists {
		idx = &Index{
			chunkMap:     make(map[string][]*Document),
			embeddings:   make(map[string][]float32),
			fileModTimes: make(map[string]time.Time),
			projectDir:   projectDir,
		}
		s.indices[projectDir] = idx
	}
	s.mu.Unlock()

	var docs []*Document
	err := filepath.Walk(projectDir, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return nil
		}
		if info.IsDir() {
			name := info.Name()
			if name == "node_modules" || name == ".git" || name == "dist" || name == "build" ||
				name == "vendor" || name == "__pycache__" || name == ".next" || name == "target" ||
				name == ".venv" || name == "venv" {
				return filepath.SkipDir
			}
			return nil
		}

		ext := strings.ToLower(filepath.Ext(path))
		supportedExts := map[string]bool{
			".go": true, ".js": true, ".ts": true, ".tsx": true, ".jsx": true,
			".py": true, ".rs": true, ".java": true, ".cpp": true, ".c": true,
			".h": true, ".cs": true, ".rb": true, ".php": true, ".sql": true,
			".md": true, ".yaml": true, ".yml": true, ".json": true, ".toml": true,
			".xml": true, ".html": true, ".css": true, ".scss": true, ".svelte": true,
		}
		if !supportedExts[ext] {
			return nil
		}

		if info.Size() > 100*1024 {
			return nil
		}

		content, err := os.ReadFile(path)
		if err != nil {
			return nil
		}

		relPath, _ := filepath.Rel(projectDir, path)
		chunks := s.chunkText(string(content), s.chunkSize, s.overlap)

		for i, chunk := range chunks {
			docID := fmt.Sprintf("%s:%d", relPath, i)
			docs = append(docs, &Document{
				ID:      docID,
				Content: chunk,
				Metadata: map[string]string{
					"path":      relPath,
					"chunk":     fmt.Sprintf("%d", i),
					"language":  extToLanguage(ext),
					"indexedAt": time.Now().Format(time.RFC3339),
				},
			})
		}
		return nil
	})
	if err != nil {
		return fmt.Errorf("walk project: %w", err)
	}

	idx.mu.Lock()
	idx.documents = docs
	idx.chunkMap = make(map[string][]*Document)
	for _, doc := range docs {
		path := doc.Metadata["path"]
		idx.chunkMap[path] = append(idx.chunkMap[path], doc)
	}
	idx.updatedAt = time.Now()
	idx.mu.Unlock()

	go s.generateEmbeddings(projectDir, docs)

	log.Printf("[RAG] Indexed %d chunks from %s", len(docs), projectDir)
	return nil
}

func (s *Service) Search(ctx context.Context, projectDir string, query string, topK int) ([]SearchResult, error) {
	if topK <= 0 {
		topK = 5
	}

	s.mu.RLock()
	idx, exists := s.indices[projectDir]
	s.mu.RUnlock()

	if !exists {
		return nil, fmt.Errorf("project not indexed: %s", projectDir)
	}

	queryEmbedding, err := s.getEmbedding(ctx, query)
	if err != nil {
		log.Printf("[RAG] Embedding failed, falling back to keyword search: %v", err)
		return s.keywordSearch(idx, query, topK), nil
	}

	idx.mu.RLock()
	defer idx.mu.RUnlock()

	var results []SearchResult
	for _, doc := range idx.documents {
		docEmbedding, hasEmbedding := idx.embeddings[doc.ID]
		if !hasEmbedding {
			continue
		}
		score := cosineSimilarity(queryEmbedding, docEmbedding)
		if score > 0.3 {
			results = append(results, SearchResult{
				Document:  doc,
				Score:     score,
				ChunkText: doc.Content,
			})
		}
	}

	sort.Slice(results, func(i, j int) bool {
		return results[i].Score > results[j].Score
	})

	if len(results) > topK {
		results = results[:topK]
	}

	if len(results) < topK {
		keywordResults := s.keywordSearch(idx, query, topK-len(results))
		existingIDs := make(map[string]bool)
		for _, r := range results {
			existingIDs[r.Document.ID] = true
		}
		for _, kr := range keywordResults {
			if !existingIDs[kr.Document.ID] {
				results = append(results, kr)
			}
		}
	}

	return results, nil
}

func (s *Service) SearchHybrid(ctx context.Context, projectDir string, query string, topK int) ([]SearchResult, error) {
	if topK <= 0 {
		topK = 5
	}

	s.mu.RLock()
	idx, exists := s.indices[projectDir]
	s.mu.RUnlock()

	if !exists {
		return nil, fmt.Errorf("project not indexed: %s", projectDir)
	}

	var semanticResults []SearchResult
	queryEmbedding, err := s.getEmbedding(ctx, query)
	if err == nil {
		idx.mu.RLock()
		for _, doc := range idx.documents {
			docEmbedding, hasEmbedding := idx.embeddings[doc.ID]
			if !hasEmbedding {
				continue
			}
			score := cosineSimilarity(queryEmbedding, docEmbedding)
			if score > 0.2 {
				semanticResults = append(semanticResults, SearchResult{
					Document:  doc,
					Score:     score * 0.7,
					ChunkText: doc.Content,
				})
			}
		}
		idx.mu.RUnlock()
	}

	keywordResults := s.keywordSearch(idx, query, topK*2)
	for i := range keywordResults {
		keywordResults[i].Score = keywordResults[i].Score * 0.3
	}

	allResults := append(semanticResults, keywordResults...)

	deduped := make(map[string]*SearchResult)
	for i := range allResults {
		id := allResults[i].Document.ID
		if existing, ok := deduped[id]; ok {
			if allResults[i].Score > existing.Score {
				deduped[id] = &allResults[i]
			}
		} else {
			deduped[id] = &allResults[i]
		}
	}

	var final []SearchResult
	for _, r := range deduped {
		final = append(final, *r)
	}
	sort.Slice(final, func(i, j int) bool {
		return final[i].Score > final[j].Score
	})

	if len(final) > topK {
		final = final[:topK]
	}

	return final, nil
}

func (s *Service) IsIndexed(projectDir string) bool {
	s.mu.RLock()
	defer s.mu.RUnlock()
	_, exists := s.indices[projectDir]
	return exists
}

func (s *Service) GetIndexStats(projectDir string) map[string]any {
	s.mu.RLock()
	idx, exists := s.indices[projectDir]
	s.mu.RUnlock()

	if !exists {
		return map[string]any{"indexed": false}
	}

	idx.mu.RLock()
	defer idx.mu.RUnlock()

	fileCount := len(idx.chunkMap)
	return map[string]any{
		"indexed":    true,
		"chunks":     len(idx.documents),
		"files":      fileCount,
		"embeddings": len(idx.embeddings),
		"updatedAt":  idx.updatedAt.Format(time.RFC3339),
	}
}

func (s *Service) generateEmbeddings(projectDir string, docs []*Document) {
	p := s.providerMgr.GetDefaultProvider()
	if p == nil {
		return
	}

	s.mu.RLock()
	idx := s.indices[projectDir]
	s.mu.RUnlock()

	if idx == nil {
		return
	}

	batchSize := 20
	for i := 0; i < len(docs); i += batchSize {
		end := i + batchSize
		if end > len(docs) {
			end = len(docs)
		}
		batch := docs[i:end]

		for _, doc := range batch {
			if _, exists := idx.embeddings[doc.ID]; exists {
				continue
			}

			ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
			embedding, err := s.getEmbedding(ctx, doc.Content)
			cancel()

			if err != nil {
				continue
			}

			idx.mu.Lock()
			idx.embeddings[doc.ID] = embedding
			idx.mu.Unlock()
		}
	}

	log.Printf("[RAG] Generated embeddings for %d/%d chunks in %s", len(idx.embeddings), len(docs), projectDir)
}

func (s *Service) getEmbedding(ctx context.Context, text string) ([]float32, error) {
	p := s.providerMgr.GetDefaultProvider()
	if p == nil {
		return nil, fmt.Errorf("no provider")
	}

	cfg := p.GetConfig()
	if cfg.APIKey == "" {
		return nil, fmt.Errorf("no API key")
	}

	if len(text) > 2000 {
		text = text[:2000]
	}

	endpoint := cfg.Endpoint
	if endpoint == "" {
		endpoint = "https://api.openai.com/v1"
	}
	endpoint = strings.TrimRight(endpoint, "/")

	embedURL := endpoint + "/embeddings"

	body := map[string]any{
		"model": "text-embedding-3-small",
		"input": text,
	}
	bodyJSON, _ := json.Marshal(body)

	req, err := newEmbeddingRequest(ctx, embedURL, bodyJSON, cfg.APIKey)
	if err != nil {
		return nil, err
	}

	resp, err := embeddingHTTPClient().Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != 200 {
		return nil, fmt.Errorf("embedding API returned %d", resp.StatusCode)
	}

	var result struct {
		Data []struct {
			Embedding []float32 `json:"embedding"`
		} `json:"data"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, err
	}

	if len(result.Data) == 0 {
		return nil, fmt.Errorf("no embedding returned")
	}

	return result.Data[0].Embedding, nil
}

func newEmbeddingRequest(ctx context.Context, url string, body []byte, apiKey string) (*http.Request, error) {
	req, err := http.NewRequestWithContext(ctx, "POST", url, strings.NewReader(string(body)))
	if err != nil {
		return nil, err
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+apiKey)
	return req, nil
}

func embeddingHTTPClient() *http.Client {
	return &http.Client{Timeout: 15 * time.Second}
}

func (s *Service) keywordSearch(idx *Index, query string, topK int) []SearchResult {
	idx.mu.RLock()
	defer idx.mu.RUnlock()

	terms := strings.Fields(strings.ToLower(query))
	var results []SearchResult

	for _, doc := range idx.documents {
		content := strings.ToLower(doc.Content)
		score := 0.0
		for _, term := range terms {
			count := strings.Count(content, term)
			if count > 0 {
				score += float64(count) / float64(len(strings.Fields(content)))
			}
		}
		if score > 0 {
			pathScore := 0.0
			pathLower := strings.ToLower(doc.Metadata["path"])
			for _, term := range terms {
				if strings.Contains(pathLower, term) {
					pathScore += 0.1
				}
			}
			results = append(results, SearchResult{
				Document:  doc,
				Score:     score + pathScore,
				ChunkText: doc.Content,
			})
		}
	}

	sort.Slice(results, func(i, j int) bool {
		return results[i].Score > results[j].Score
	})

	if len(results) > topK {
		results = results[:topK]
	}

	return results
}

func (s *Service) chunkText(text string, chunkSize int, overlap int) []string {
	if text == "" {
		return nil
	}

	lines := strings.Split(text, "\n")
	var chunks []string
	var current strings.Builder
	currentLines := 0

	for _, line := range lines {
		current.WriteString(line)
		current.WriteString("\n")
		currentLines++

		if currentLines >= chunkSize/30 {
			chunks = append(chunks, current.String())
			if overlap > 0 && currentLines > overlap/30 {
				recentLines := strings.Split(current.String(), "\n")
				keepFrom := len(recentLines) - overlap/30
				if keepFrom < 0 {
					keepFrom = 0
				}
				current.Reset()
				for i := keepFrom; i < len(recentLines); i++ {
					current.WriteString(recentLines[i])
					current.WriteString("\n")
				}
				currentLines = len(recentLines) - keepFrom
			} else {
				current.Reset()
				currentLines = 0
			}
		}
	}

	if current.Len() > 0 {
		chunks = append(chunks, current.String())
	}

	return chunks
}

func cosineSimilarity(a, b []float32) float64 {
	if len(a) != len(b) || len(a) == 0 {
		return 0
	}
	var dotProduct, normA, normB float64
	for i := range a {
		dotProduct += float64(a[i]) * float64(b[i])
		normA += float64(a[i]) * float64(a[i])
		normB += float64(b[i]) * float64(b[i])
	}
	if normA == 0 || normB == 0 {
		return 0
	}
	return dotProduct / (math.Sqrt(normA) * math.Sqrt(normB))
}

func extToLanguage(ext string) string {
	switch ext {
	case ".go":
		return "go"
	case ".js", ".jsx", ".mjs":
		return "javascript"
	case ".ts", ".tsx":
		return "typescript"
	case ".py":
		return "python"
	case ".rs":
		return "rust"
	case ".java":
		return "java"
	case ".cpp", ".c", ".h":
		return "cpp"
	case ".cs":
		return "csharp"
	case ".rb":
		return "ruby"
	case ".php":
		return "php"
	case ".sql":
		return "sql"
	case ".html":
		return "html"
	case ".css", ".scss":
		return "css"
	case ".svelte":
		return "svelte"
	case ".md":
		return "markdown"
	case ".yaml", ".yml":
		return "yaml"
	case ".json":
		return "json"
	case ".xml":
		return "xml"
	default:
		return "unknown"
	}
}

func (s *Service) IndexFileIncremental(ctx context.Context, projectDir string, filePath string) error {
	s.mu.Lock()
	idx, exists := s.indices[projectDir]
	if !exists {
		s.mu.Unlock()
		return s.IndexProject(ctx, projectDir)
	}
	s.mu.Unlock()

	info, err := os.Stat(filePath)
	if err != nil {
		return fmt.Errorf("stat file: %w", err)
	}

	relPath, _ := filepath.Rel(projectDir, filePath)
	idx.mu.RLock()
	lastMod, hasMod := idx.fileModTimes[relPath]
	idx.mu.RUnlock()

	if hasMod && !info.ModTime().After(lastMod) {
		return nil
	}

	ext := strings.ToLower(filepath.Ext(filePath))
	supportedExts := map[string]bool{
		".go": true, ".js": true, ".ts": true, ".tsx": true, ".jsx": true,
		".py": true, ".rs": true, ".java": true, ".cpp": true, ".c": true,
		".h": true, ".cs": true, ".rb": true, ".php": true, ".sql": true,
		".md": true, ".yaml": true, ".yml": true, ".json": true, ".toml": true,
		".xml": true, ".html": true, ".css": true, ".scss": true, ".svelte": true,
	}
	if !supportedExts[ext] {
		return nil
	}

	content, err := os.ReadFile(filePath)
	if err != nil {
		return fmt.Errorf("read file: %w", err)
	}

	chunks := s.chunkText(string(content), s.chunkSize, s.overlap)
	var newDocs []*Document
	for i, chunk := range chunks {
		docID := fmt.Sprintf("%s:%d", relPath, i)
		newDocs = append(newDocs, &Document{
			ID:      docID,
			Content: chunk,
			Metadata: map[string]string{
				"path":      relPath,
				"chunk":     fmt.Sprintf("%d", i),
				"language":  extToLanguage(ext),
				"indexedAt": time.Now().Format(time.RFC3339),
			},
		})
	}

	idx.mu.Lock()
	delete(idx.chunkMap, relPath)
	var filtered []*Document
	for _, doc := range idx.documents {
		if doc.Metadata["path"] != relPath {
			filtered = append(filtered, doc)
		}
	}
	delete(idx.embeddings, relPath)
	filtered = append(filtered, newDocs...)
	idx.documents = filtered
	for _, doc := range newDocs {
		idx.chunkMap[relPath] = append(idx.chunkMap[relPath], doc)
	}
	idx.fileModTimes[relPath] = info.ModTime()
	idx.updatedAt = time.Now()
	idx.mu.Unlock()

	log.Printf("[RAG] incremental index: %s (%d chunks)", relPath, len(newDocs))
	return nil
}

func (s *Service) RemoveFileFromIndex(projectDir string, filePath string) {
	s.mu.RLock()
	idx, exists := s.indices[projectDir]
	s.mu.RUnlock()
	if !exists {
		return
	}

	relPath, _ := filepath.Rel(projectDir, filePath)
	idx.mu.Lock()
	defer idx.mu.Unlock()

	delete(idx.chunkMap, relPath)
	delete(idx.embeddings, relPath)
	delete(idx.fileModTimes, relPath)

	var filtered []*Document
	for _, doc := range idx.documents {
		if doc.Metadata["path"] != relPath {
			filtered = append(filtered, doc)
		}
	}
	idx.documents = filtered
}
