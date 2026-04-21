// PicoClaw - Ultra-lightweight personal AI agent
// Inspired by and based on nanobot: https://github.com/HKUDS/nanobot
// License: MIT
//
// Copyright (c) 2026 PicoClaw contributors

package agent

import (
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"github.com/sipeed/picoclaw/pkg/config"
	"github.com/sipeed/picoclaw/pkg/fileutil"
)

// MemoryStore manages persistent memory for the agent.
// - Long-term memory: memory/MEMORY.md
// - Daily notes: memory/YYYYMM/YYYYMMDD.md
type MemoryStore struct {
	workspace  string
	memoryDir  string
	memoryFile string
	coreConfig config.MemoryCoreConfig
}

// NewMemoryStore creates a new MemoryStore with the given workspace path.
// It ensures the memory directory exists.
func NewMemoryStore(workspace string) *MemoryStore {
	memoryDir := filepath.Join(workspace, "memory")
	memoryFile := filepath.Join(memoryDir, "MEMORY.md")

	// Ensure memory directory exists
	os.MkdirAll(memoryDir, 0o755)

	return &MemoryStore{
		workspace:  workspace,
		memoryDir:  memoryDir,
		memoryFile: memoryFile,
	}
}

// WithMemoryCore enables optional integration with Project-AI-MemoryCore.
func (ms *MemoryStore) WithMemoryCore(cfg config.MemoryCoreConfig) *MemoryStore {
	ms.coreConfig = cfg
	if ms.coreConfig.MaxDiaryFiles <= 0 {
		ms.coreConfig.MaxDiaryFiles = 2
	}
	return ms
}

// getTodayFile returns the path to today's daily note file (memory/YYYYMM/YYYYMMDD.md).
func (ms *MemoryStore) getTodayFile() string {
	today := time.Now().Format("20060102") // YYYYMMDD
	monthDir := today[:6]                  // YYYYMM
	filePath := filepath.Join(ms.memoryDir, monthDir, today+".md")
	return filePath
}

// ReadLongTerm reads the long-term memory (MEMORY.md).
// Returns empty string if the file doesn't exist.
func (ms *MemoryStore) ReadLongTerm() string {
	if data, err := os.ReadFile(ms.memoryFile); err == nil {
		return string(data)
	}
	return ""
}

// WriteLongTerm writes content to the long-term memory file (MEMORY.md).
func (ms *MemoryStore) WriteLongTerm(content string) error {
	// Use unified atomic write utility with explicit sync for flash storage reliability.
	// Using 0o600 (owner read/write only) for secure default permissions.
	return fileutil.WriteFileAtomic(ms.memoryFile, []byte(content), 0o600)
}

// ReadToday reads today's daily note.
// Returns empty string if the file doesn't exist.
func (ms *MemoryStore) ReadToday() string {
	todayFile := ms.getTodayFile()
	if data, err := os.ReadFile(todayFile); err == nil {
		return string(data)
	}
	return ""
}

// AppendToday appends content to today's daily note.
// If the file doesn't exist, it creates a new file with a date header.
func (ms *MemoryStore) AppendToday(content string) error {
	todayFile := ms.getTodayFile()

	// Ensure month directory exists
	monthDir := filepath.Dir(todayFile)
	if err := os.MkdirAll(monthDir, 0o755); err != nil {
		return err
	}

	var existingContent string
	if data, err := os.ReadFile(todayFile); err == nil {
		existingContent = string(data)
	}

	var newContent string
	if existingContent == "" {
		// Add header for new day
		header := fmt.Sprintf("# %s\n\n", time.Now().Format("2006-01-02"))
		newContent = header + content
	} else {
		// Append to existing content
		newContent = existingContent + "\n" + content
	}

	// Use unified atomic write utility with explicit sync for flash storage reliability.
	return fileutil.WriteFileAtomic(todayFile, []byte(newContent), 0o600)
}

// GetRecentDailyNotes returns daily notes from the last N days.
// Contents are joined with "---" separator.
func (ms *MemoryStore) GetRecentDailyNotes(days int) string {
	var sb strings.Builder
	first := true

	for i := range days {
		date := time.Now().AddDate(0, 0, -i)
		dateStr := date.Format("20060102") // YYYYMMDD
		monthDir := dateStr[:6]            // YYYYMM
		filePath := filepath.Join(ms.memoryDir, monthDir, dateStr+".md")

		if data, err := os.ReadFile(filePath); err == nil {
			if !first {
				sb.WriteString("\n\n---\n\n")
			}
			sb.Write(data)
			first = false
		}
	}

	return sb.String()
}

// GetMemoryContext returns formatted memory context for the agent prompt.
// Includes long-term memory and recent daily notes.
func (ms *MemoryStore) GetMemoryContext() string {
	longTerm := ms.ReadLongTerm()
	recentNotes := ms.GetRecentDailyNotes(3)
	coreContext := ms.GetMemoryCoreContext()

	if longTerm == "" && recentNotes == "" && coreContext == "" {
		return ""
	}

	var sb strings.Builder

	if longTerm != "" {
		sb.WriteString("## Long-term Memory\n\n")
		sb.WriteString(longTerm)
	}

	if recentNotes != "" {
		if longTerm != "" {
			sb.WriteString("\n\n---\n\n")
		}
		sb.WriteString("## Recent Daily Notes\n\n")
		sb.WriteString(recentNotes)
	}

	if coreContext != "" {
		if longTerm != "" || recentNotes != "" {
			sb.WriteString("\n\n---\n\n")
		}
		sb.WriteString(coreContext)
	}

	return sb.String()
}

func (ms *MemoryStore) GetMemoryCoreContext() string {
	if !ms.coreConfig.Enabled {
		return ""
	}

	corePath := strings.TrimSpace(ms.coreConfig.Path)
	if corePath == "" {
		corePath = filepath.Join(ms.workspace, "ai-memorycore")
	}

	readCore := func(rel string) string {
		path := filepath.Join(corePath, rel)
		data, err := os.ReadFile(path)
		if err != nil {
			return ""
		}
		content := strings.TrimSpace(string(data))
		if content == "" {
			return ""
		}
		const maxBytes = 16 * 1024
		if len(content) > maxBytes {
			content = content[:maxBytes] + "\n\n[truncated]"
		}
		return content
	}

	var sections []string
	for _, file := range []struct {
		Title string
		Path  string
	}{
		{Title: "MemoryCore Master", Path: "master-memory.md"},
		{Title: "MemoryCore Identity", Path: filepath.Join("main", "identity-core.md")},
		{Title: "MemoryCore Relationship", Path: filepath.Join("main", "relationship-memory.md")},
		{Title: "MemoryCore Current Session", Path: filepath.Join("main", "current-session.md")},
	} {
		if content := readCore(file.Path); content != "" {
			sections = append(sections, "### "+file.Title+"\n\n"+content)
		}
	}

	if diary := ms.readRecentMemoryCoreDiaries(corePath); diary != "" {
		sections = append(sections, "### MemoryCore Daily Diary\n\n"+diary)
	}

	if len(sections) == 0 {
		return ""
	}

	return "## MemoryCore Context\n\n" + strings.Join(sections, "\n\n---\n\n")
}

func (ms *MemoryStore) readRecentMemoryCoreDiaries(corePath string) string {
	diaryDir := filepath.Join(corePath, "daily-diary")
	entries, err := os.ReadDir(diaryDir)
	if err != nil {
		return ""
	}

	var names []string
	for _, entry := range entries {
		if entry.IsDir() {
			continue
		}
		name := entry.Name()
		if strings.HasSuffix(strings.ToLower(name), ".md") {
			names = append(names, name)
		}
	}

	if len(names) == 0 {
		return ""
	}

	sort.Strings(names)
	maxFiles := ms.coreConfig.MaxDiaryFiles
	if maxFiles <= 0 {
		maxFiles = 2
	}
	if len(names) > maxFiles {
		names = names[len(names)-maxFiles:]
	}

	var chunks []string
	for _, name := range names {
		path := filepath.Join(diaryDir, name)
		data, err := os.ReadFile(path)
		if err != nil {
			continue
		}
		content := strings.TrimSpace(string(data))
		if content == "" {
			continue
		}
		const maxBytes = 8 * 1024
		if len(content) > maxBytes {
			content = content[:maxBytes] + "\n\n[truncated]"
		}
		chunks = append(chunks, "#### "+name+"\n\n"+content)
	}

	return strings.Join(chunks, "\n\n")
}
