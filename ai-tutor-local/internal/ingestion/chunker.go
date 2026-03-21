package ingestion

import (
	"strings"
)

// ChunkResult is a semantic chunk output from raw document text.
type ChunkResult struct {
	ChapterName   string
	ChunkIndex    int
	Content       string
	TaggedContent string
	TokenCount    int
}

// Chunker performs simple token-aware chunking with overlap.
type Chunker struct {
	maxTokens int
	overlap   int
}

func NewChunker(maxTokens, overlap int) *Chunker {
	if maxTokens <= 0 {
		maxTokens = 400
	}
	if overlap < 0 {
		overlap = 50
	}
	return &Chunker{maxTokens: maxTokens, overlap: overlap}
}

// ChunkText creates chunks from text by word windows.
func (c *Chunker) ChunkText(notebookName, chapterName, text string) []ChunkResult {
	words := strings.Fields(text)
	if len(words) == 0 {
		return nil
	}

	stride := c.maxTokens - c.overlap
	if stride <= 0 {
		stride = c.maxTokens
	}

	chunks := make([]ChunkResult, 0)
	idx := 0
	for start := 0; start < len(words); start += stride {
		end := start + c.maxTokens
		if end > len(words) {
			end = len(words)
		}

		content := strings.Join(words[start:end], " ")
		tagged := "[" + notebookName + " - " + chapterName + "] " + content
		chunks = append(chunks, ChunkResult{
			ChapterName:   chapterName,
			ChunkIndex:    idx,
			Content:       content,
			TaggedContent: tagged,
			TokenCount:    end - start,
		})
		idx++

		if end == len(words) {
			break
		}
	}

	return chunks
}
