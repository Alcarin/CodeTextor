package utils

import (
	"bufio"
	"os"
	"strings"
)

// Chunk represents a piece of text from a file.
type Chunk struct {
	Content      string
	LineStart    int
	LineEnd      int
	CharacterStart int
	CharacterEnd int
}

// ChunkFile reads a file and splits it into chunks based on the provided configuration.
func ChunkFile(filePath string, chunkSize int) ([]Chunk, error) {
	file, err := os.Open(filePath)
	if err != nil {
		return nil, err
	}
	defer file.Close()

	var chunks []Chunk
	scanner := bufio.NewScanner(file)
	var currentChunk strings.Builder
	var lineStart, charOffset int

	for lineNum := 1; scanner.Scan(); lineNum++ {
		line := scanner.Text()
		if currentChunk.Len()+len(line) > chunkSize && currentChunk.Len() > 0 {
			chunks = append(chunks, Chunk{
				Content:      currentChunk.String(),
				LineStart:    lineStart,
				LineEnd:      lineNum - 1,
				CharacterStart: charOffset - currentChunk.Len(),
				CharacterEnd:   charOffset,
			})
			currentChunk.Reset()
		}
		if currentChunk.Len() == 0 {
			lineStart = lineNum
		}
		currentChunk.WriteString(line)
		currentChunk.WriteString("\n")
		charOffset += len(line) + 1 // +1 for newline character
	}

	if currentChunk.Len() > 0 {
		chunks = append(chunks, Chunk{
			Content:      currentChunk.String(),
			LineStart:    lineStart,
			LineEnd:      -1, // Indicates to the end of the file
			CharacterStart: charOffset - currentChunk.Len(),
			CharacterEnd:   charOffset,
		})
	}

	if err := scanner.Err(); err != nil {
		return nil, err
	}

	return chunks, nil
}
