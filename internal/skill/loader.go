package skill

import (
	"fmt"
	"io/ioutil"
	"os"
	"path/filepath"
	"strings"
)

// LoadSkillsFromDir scans a directory for SKILL.md files and returns SkillDef entries.
// Each subdirectory containing SKILL.md becomes a skill.
func LoadSkillsFromDir(dir string) []SkillDef {
	var skills []SkillDef

	entries, err := ioutil.ReadDir(dir)
	if err != nil {
		return skills
	}

	for _, entry := range entries {
		if !entry.IsDir() || strings.HasPrefix(entry.Name(), ".") {
			continue
		}

		skillPath := filepath.Join(dir, entry.Name())
		skillFile := filepath.Join(skillPath, "SKILL.md")
		data, err := ioutil.ReadFile(skillFile)
		if err != nil {
			continue
		}

		content := string(data)
		id := entry.Name()
		fm, body := parseFrontmatter(content)
		name := fm["name"]
		if name == "" {
			name = extractName(body, id)
		}
		desc := fm["description"]
		if desc == "" {
			desc = extractDescription(body)
		}
		icon := extractIcon(body)

		skills = append(skills, SkillDef{
			ID:               id,
			Name:             name,
			Icon:             icon,
			Category:         "external",
			Trigger:          "manual",
			ResultType:       "text",
			Description:      desc,
			PromptTemplate:   body,
			AssociatedAgents: []string{"universal-assistant"},
		})
	}

	return skills
}

// parseFrontmatter extracts YAML-style frontmatter (between --- markers) from content.
// Returns a map of key-value pairs and the content body without the frontmatter.
func parseFrontmatter(content string) (map[string]string, string) {
	fm := make(map[string]string)
	lines := strings.Split(content, "\n")
	if len(lines) == 0 || strings.TrimSpace(lines[0]) != "---" {
		return fm, content
	}
	endIdx := -1
	for i := 1; i < len(lines); i++ {
		if strings.TrimSpace(lines[i]) == "---" {
			endIdx = i
			break
		}
		parts := strings.SplitN(strings.TrimSpace(lines[i]), ":", 2)
		if len(parts) == 2 {
			fm[strings.TrimSpace(parts[0])] = strings.TrimSpace(parts[1])
		}
	}
	if endIdx < 0 {
		return fm, content
	}
	body := strings.Join(lines[endIdx+1:], "\n")
	body = strings.TrimSpace(body)
	return fm, body
}

func extractName(content, fallback string) string {
	lines := strings.Split(content, "\n")
	for _, line := range lines {
		line = strings.TrimSpace(line)
		if strings.HasPrefix(line, "# ") {
			return strings.TrimPrefix(line, "# ")
		}
	}
	name := strings.ReplaceAll(fallback, "-", " ")
	// Use manual title-case conversion instead of deprecated strings.Title
	words := strings.Fields(name)
	for i, w := range words {
		if len(w) > 0 {
			words[i] = strings.ToUpper(w[:1]) + w[1:]
		}
	}
	return strings.Join(words, " ")
}

func extractDescription(content string) string {
	lines := strings.Split(content, "\n")
	for i, line := range lines {
		line = strings.TrimSpace(line)
		if strings.HasPrefix(line, "> ") {
			return strings.TrimPrefix(line, "> ")
		}
		if i > 0 && line != "" && !strings.HasPrefix(line, "#") {
			if len(line) > 120 {
				line = line[:120] + "..."
			}
			return line
		}
	}
	return "External skill"
}

func extractIcon(content string) string {
	lines := strings.Split(content, "\n")
	for _, line := range lines {
		line = strings.TrimSpace(line)
		if strings.HasPrefix(line, "# ") {
			title := strings.TrimPrefix(line, "# ")
			for _, r := range title {
				if r >= 0x1F300 && r <= 0x1F9FF || r >= 0x2600 && r <= 0x27BF {
					return string(r)
				}
			}
		}
	}
	return "📋"
}

// GetSkillsDir returns the path where external skills are stored
func GetSkillsDir(configDir string) string {
	dir := filepath.Join(configDir, "skills")
	os.MkdirAll(dir, 0755)
	return dir
}

// BuildSkillMarkdown generates SKILL.md content from a SkillDef.
func BuildSkillMarkdown(s SkillDef) string {
	var sb strings.Builder
	sb.WriteString("---\n")
	if s.Name != "" {
		sb.WriteString(fmt.Sprintf("name: %s\n", s.Name))
	}
	if s.Description != "" {
		sb.WriteString(fmt.Sprintf("description: %s\n", s.Description))
	}
	sb.WriteString("---\n\n")
	sb.WriteString("# " + s.Icon + " " + s.Name + "\n\n")
	sb.WriteString(s.PromptTemplate)
	return sb.String()
}
