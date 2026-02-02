// Package embeddings provides infrastructure for the embeddings service.
package embeddings

import (
	"regexp"
	"strings"

	domainEmbeddings "github.com/anthropics/claude-flow-go/internal/domain/embeddings"
)

// Chunker provides document chunking functionality.
type Chunker struct {
	config domainEmbeddings.ChunkingConfig
}

// NewChunker creates a new chunker with the given configuration.
func NewChunker(config domainEmbeddings.ChunkingConfig) *Chunker {
	if config.MaxChunkSize <= 0 {
		config.MaxChunkSize = 512
	}
	if config.Overlap < 0 {
		config.Overlap = 50
	}
	if config.MinChunkSize <= 0 {
		config.MinChunkSize = 100
	}
	if config.Strategy == "" {
		config.Strategy = domainEmbeddings.ChunkingSentence
	}

	return &Chunker{config: config}
}

// Chunk splits a document into chunks.
func (c *Chunker) Chunk(text string) *domainEmbeddings.ChunkedDocument {
	if text == "" {
		return &domainEmbeddings.ChunkedDocument{
			Chunks:         nil,
			OriginalLength: 0,
			TotalChunks:    0,
			Config:         c.config,
		}
	}

	var chunks []domainEmbeddings.Chunk

	switch c.config.Strategy {
	case domainEmbeddings.ChunkingCharacter:
		chunks = c.chunkByCharacter(text)
	case domainEmbeddings.ChunkingSentence:
		chunks = c.chunkBySentence(text)
	case domainEmbeddings.ChunkingParagraph:
		chunks = c.chunkByParagraph(text)
	case domainEmbeddings.ChunkingToken:
		chunks = c.chunkByToken(text)
	default:
		chunks = c.chunkBySentence(text)
	}

	return &domainEmbeddings.ChunkedDocument{
		Chunks:         chunks,
		OriginalLength: len(text),
		TotalChunks:    len(chunks),
		Config:         c.config,
	}
}

// chunkByCharacter splits by character count with overlap.
func (c *Chunker) chunkByCharacter(text string) []domainEmbeddings.Chunk {
	if len(text) <= c.config.MaxChunkSize {
		return []domainEmbeddings.Chunk{{
			Text:       text,
			Index:      0,
			StartPos:   0,
			EndPos:     len(text),
			Length:     len(text),
			TokenCount: estimateTokens(text),
		}}
	}

	var chunks []domainEmbeddings.Chunk
	step := c.config.MaxChunkSize - c.config.Overlap
	if step <= 0 {
		step = c.config.MaxChunkSize / 2
	}

	for start := 0; start < len(text); start += step {
		end := start + c.config.MaxChunkSize
		if end > len(text) {
			end = len(text)
		}

		chunkText := text[start:end]
		if len(chunkText) < c.config.MinChunkSize && len(chunks) > 0 {
			// Merge with previous chunk if too small
			break
		}

		chunks = append(chunks, domainEmbeddings.Chunk{
			Text:       chunkText,
			Index:      len(chunks),
			StartPos:   start,
			EndPos:     end,
			Length:     len(chunkText),
			TokenCount: estimateTokens(chunkText),
		})

		if end >= len(text) {
			break
		}
	}

	return chunks
}

// Sentence boundary pattern
var sentencePattern = regexp.MustCompile(`[.!?]+\s+`)

// chunkBySentence splits by sentence boundaries.
func (c *Chunker) chunkBySentence(text string) []domainEmbeddings.Chunk {
	if len(text) <= c.config.MaxChunkSize {
		return []domainEmbeddings.Chunk{{
			Text:       text,
			Index:      0,
			StartPos:   0,
			EndPos:     len(text),
			Length:     len(text),
			TokenCount: estimateTokens(text),
		}}
	}

	// Find sentence boundaries
	boundaries := sentencePattern.FindAllStringIndex(text, -1)
	if len(boundaries) == 0 {
		// No sentence boundaries, fall back to character chunking
		return c.chunkByCharacter(text)
	}

	// Extract sentence end positions
	sentenceEnds := make([]int, len(boundaries)+1)
	for i, boundary := range boundaries {
		sentenceEnds[i] = boundary[1]
	}
	sentenceEnds[len(sentenceEnds)-1] = len(text)

	var chunks []domainEmbeddings.Chunk
	start := 0

	for i := 0; i < len(sentenceEnds); {
		// Build chunk from sentences
		end := sentenceEnds[i]
		
		// Keep adding sentences while under max size
		for i < len(sentenceEnds)-1 {
			nextEnd := sentenceEnds[i+1]
			if nextEnd-start > c.config.MaxChunkSize {
				break
			}
			end = nextEnd
			i++
		}

		chunkText := text[start:end]
		chunkText = strings.TrimSpace(chunkText)

		if len(chunkText) > 0 {
			chunks = append(chunks, domainEmbeddings.Chunk{
				Text:       chunkText,
				Index:      len(chunks),
				StartPos:   start,
				EndPos:     end,
				Length:     len(chunkText),
				TokenCount: estimateTokens(chunkText),
			})
		}

		// Apply overlap
		if c.config.Overlap > 0 && i < len(sentenceEnds)-1 {
			// Move back by overlap amount
			overlapStart := end - c.config.Overlap
			if overlapStart < start {
				overlapStart = start
			}
			// Find sentence boundary before overlap point
			for j := i - 1; j >= 0 && sentenceEnds[j] > overlapStart; j-- {
				if sentenceEnds[j] < end {
					start = sentenceEnds[j]
					break
				}
			}
		} else {
			start = end
		}
		i++
	}

	return chunks
}

// Paragraph pattern
var paragraphPattern = regexp.MustCompile(`\n\n+`)

// chunkByParagraph splits by paragraph boundaries.
func (c *Chunker) chunkByParagraph(text string) []domainEmbeddings.Chunk {
	if len(text) <= c.config.MaxChunkSize {
		return []domainEmbeddings.Chunk{{
			Text:       text,
			Index:      0,
			StartPos:   0,
			EndPos:     len(text),
			Length:     len(text),
			TokenCount: estimateTokens(text),
		}}
	}

	// Split by paragraphs
	paragraphs := paragraphPattern.Split(text, -1)
	if len(paragraphs) == 0 {
		return c.chunkBySentence(text)
	}

	var chunks []domainEmbeddings.Chunk
	currentChunk := strings.Builder{}
	currentStart := 0
	pos := 0

	for _, para := range paragraphs {
		para = strings.TrimSpace(para)
		if para == "" {
			pos += 2 // Account for paragraph break
			continue
		}

		// Check if adding this paragraph exceeds max size
		if currentChunk.Len() > 0 && currentChunk.Len()+len(para)+2 > c.config.MaxChunkSize {
			// Save current chunk
			chunkText := currentChunk.String()
			chunks = append(chunks, domainEmbeddings.Chunk{
				Text:       chunkText,
				Index:      len(chunks),
				StartPos:   currentStart,
				EndPos:     pos,
				Length:     len(chunkText),
				TokenCount: estimateTokens(chunkText),
			})

			currentChunk.Reset()
			currentStart = pos
		}

		if currentChunk.Len() > 0 {
			currentChunk.WriteString("\n\n")
		}
		currentChunk.WriteString(para)
		pos += len(para) + 2
	}

	// Add remaining chunk
	if currentChunk.Len() > 0 {
		chunkText := currentChunk.String()
		chunks = append(chunks, domainEmbeddings.Chunk{
			Text:       chunkText,
			Index:      len(chunks),
			StartPos:   currentStart,
			EndPos:     len(text),
			Length:     len(chunkText),
			TokenCount: estimateTokens(chunkText),
		})
	}

	return chunks
}

// chunkByToken splits by approximate token count.
func (c *Chunker) chunkByToken(text string) []domainEmbeddings.Chunk {
	// Approximate: 4 characters per token
	maxChars := c.config.MaxChunkSize * 4
	overlapChars := c.config.Overlap * 4
	minChars := c.config.MinChunkSize * 4

	tempConfig := c.config
	tempConfig.MaxChunkSize = maxChars
	tempConfig.Overlap = overlapChars
	tempConfig.MinChunkSize = minChars
	tempConfig.Strategy = domainEmbeddings.ChunkingCharacter

	tempChunker := &Chunker{config: tempConfig}
	return tempChunker.chunkByCharacter(text)
}

// estimateTokens estimates the token count for text.
// Approximation: ~4 characters per token for English text.
func estimateTokens(text string) int {
	count := len(text) / 4
	if count == 0 && len(text) > 0 {
		count = 1
	}
	return count
}

// ========================================================================
// Helper Functions
// ========================================================================

// ChunkText is a convenience function to chunk text with default settings.
func ChunkText(text string, maxChunkSize int) *domainEmbeddings.ChunkedDocument {
	config := domainEmbeddings.DefaultChunkingConfig()
	if maxChunkSize > 0 {
		config.MaxChunkSize = maxChunkSize
	}
	return NewChunker(config).Chunk(text)
}

// ChunkForEmbedding chunks text optimized for embedding generation.
func ChunkForEmbedding(text string) *domainEmbeddings.ChunkedDocument {
	config := domainEmbeddings.ChunkingConfig{
		MaxChunkSize:    512,
		Overlap:         50,
		Strategy:        domainEmbeddings.ChunkingSentence,
		MinChunkSize:    100,
		IncludeMetadata: true,
	}
	return NewChunker(config).Chunk(text)
}

// GetChunkTexts extracts text content from chunks.
func GetChunkTexts(chunks []domainEmbeddings.Chunk) []string {
	texts := make([]string, len(chunks))
	for i, chunk := range chunks {
		texts[i] = chunk.Text
	}
	return texts
}
