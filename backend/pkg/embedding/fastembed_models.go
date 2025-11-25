package embedding

import (
	"CodeTextor/backend/pkg/models"
	"fmt"
	"strings"

	fastembed "github.com/anush008/fastembed-go"
)

func mapFastEmbedModel(meta *models.EmbeddingModelInfo) (fastembed.EmbeddingModel, error) {
	if meta == nil {
		return fastembed.BGESmallENV15, nil
	}
	key := strings.ToLower(strings.TrimSpace(meta.ID))
	switch key {
	case "fastembed/bge-small-en-v1.5", "fast-bge-small-en-v1.5", "bge-small-en-v1.5":
		return fastembed.BGESmallENV15, nil
	case "fastembed/bge-small-en", "fast-bge-small-en", "bge-small-en":
		return fastembed.BGESmallEN, nil
	case "fastembed/bge-base-en-v1.5", "fast-bge-base-en-v1.5", "bge-base-en-v1.5":
		return fastembed.BGEBaseENV15, nil
	case "fastembed/bge-base-en", "fast-bge-base-en", "bge-base-en":
		return fastembed.BGEBaseEN, nil
	case "fastembed/all-minilm-l6-v2", "fastembed/gte-small", "all-minilm-l6-v2":
		return fastembed.AllMiniLML6V2, nil
	case "fastembed/bge-small-zh-v1.5", "fast-bge-small-zh-v1.5", "bge-small-zh-v1.5":
		return fastembed.BGESmallZH, nil
	default:
		return fastembed.BGESmallENV15, fmt.Errorf("fastembed model %s is not supported", meta.ID)
	}
}

func fastEmbedHuggingFaceBase(meta *models.EmbeddingModelInfo) string {
	if meta == nil {
		return ""
	}
	switch strings.ToLower(strings.TrimSpace(meta.ID)) {
	case "fastembed/bge-small-en-v1.5", "fastembed/bge-small-en":
		return "https://huggingface.co/BAAI/bge-small-en-v1.5/resolve/main"
	case "fastembed/bge-base-en-v1.5", "fastembed/bge-base-en":
		return "https://huggingface.co/BAAI/bge-base-en-v1.5/resolve/main"
	case "fastembed/bge-small-zh-v1.5":
		return "https://huggingface.co/BAAI/bge-small-zh-v1.5/resolve/main"
	case "fastembed/gte-small":
		return "https://huggingface.co/thenlper/gte-small/resolve/main"
	case "fastembed/all-minilm-l6-v2":
		return "https://huggingface.co/sentence-transformers/all-MiniLM-L6-v2/resolve/main"
	default:
		return ""
	}
}
