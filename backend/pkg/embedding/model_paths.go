package embedding

import (
	"fmt"
	"path/filepath"
	"strings"

	"CodeTextor/backend/pkg/models"
	"CodeTextor/backend/pkg/utils"
)

// SanitizeModelID converts a model ID into a filesystem-safe directory name.
func SanitizeModelID(id string) string {
	clean := strings.TrimSpace(id)
	clean = strings.ReplaceAll(clean, "..", "")
	clean = strings.ReplaceAll(clean, "/", "-")
	clean = strings.ReplaceAll(clean, "\\", "-")
	if clean == "" {
		return "model"
	}
	return clean
}

// DefaultModelFilename resolves the expected filename for an ONNX/GGUF model.
func DefaultModelFilename(meta *models.EmbeddingModelInfo) string {
	if meta == nil {
		return "model.onnx"
	}
	if meta.PreferredFilename != "" {
		return meta.PreferredFilename
	}
	if strings.EqualFold(meta.SourceType, "gguf") {
		return "model.gguf"
	}
	return "model.onnx"
}

// ResolveModelPath returns the expected path to the model file inside the models directory.
func ResolveModelPath(meta *models.EmbeddingModelInfo) (string, error) {
	if meta == nil || strings.TrimSpace(meta.ID) == "" {
		return "", fmt.Errorf("embedding model metadata missing id")
	}
	modelsDir, err := utils.GetModelsDir()
	if err != nil {
		return "", err
	}
	sanitized := SanitizeModelID(meta.ID)
	filename := DefaultModelFilename(meta)
	targetDir := filepath.Join(modelsDir, sanitized)
	return filepath.Join(targetDir, filename), nil
}

// fastEmbedCacheDir returns the cache path used by fastembed-go.
func fastEmbedCacheDir() (string, error) {
	modelsDir, err := utils.GetModelsDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(modelsDir, "fastembed"), nil
}

// ResolveFastEmbedDir returns the expected directory containing the fastembed assets.
func ResolveFastEmbedDir(meta *models.EmbeddingModelInfo) (string, error) {
	if meta == nil {
		return "", fmt.Errorf("embedding model metadata missing")
	}
	modelID, err := mapFastEmbedModel(meta)
	if err != nil {
		return "", err
	}
	cacheDir, err := fastEmbedCacheDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(cacheDir, string(modelID)), nil
}
