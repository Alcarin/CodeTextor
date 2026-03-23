package embedding

import (
	"archive/tar"
	"archive/zip"
	"compress/gzip"
	"crypto/sha256"
	"crypto/sha512"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"path/filepath"
	"runtime"
	"strings"

	"CodeTextor/backend/pkg/models"
	"CodeTextor/backend/pkg/utils"

	_ "embed"
)

//go:embed runtime_manifest.json
var runtimeManifestData []byte

type RuntimeAsset struct {
	Name         string   `json:"name"`
	URL          string   `json:"url"`
	Hash         string   `json:"hash"`
	HashType     string   `json:"hash_type"`
	ExtractFiles []string `json:"extract_files"`
}

type RuntimeManifest struct {
	Version string                    `json:"version"`
	Assets  map[string][]RuntimeAsset `json:"assets"`
}

// DownloadProgress represents the current state of a model download.
type DownloadProgress struct {
	ModelID    string `json:"modelId"`
	Stage      string `json:"stage"`
	Downloaded int64  `json:"downloaded"`
	Total      int64  `json:"total"`
}

// DownloadProgressCallback receives progress updates for a download.
type DownloadProgressCallback func(p DownloadProgress)

func (d *Downloader) DownloadONNXRuntime(progress DownloadProgressCallback) error {
	var manifest RuntimeManifest
	if err := json.Unmarshal(runtimeManifestData, &manifest); err != nil {
		return fmt.Errorf("failed to parse embedded manifest: %w", err)
	}

	platform := fmt.Sprintf("%s-%s", runtime.GOOS, runtime.GOARCH)
	assets, ok := manifest.Assets[platform]
	if !ok {
		return fmt.Errorf("unsupported platform: %s", platform)
	}

	log.Printf("[Downloader] Platform identified: %s. %d assets to download.", platform, len(assets))

	configDir, err := utils.GetConfigDir()
	if err != nil {
		return err
	}
	binDir := filepath.Join(configDir, "bin")
	if err := os.MkdirAll(binDir, 0755); err != nil {
		return err
	}

	for _, asset := range assets {
		tempPath := filepath.Join(os.TempDir(), asset.Name+".download")
		log.Printf("[Downloader] Downloading %s from %s...", asset.Name, asset.URL)
		if err := downloadFileWithProgress(asset.Name, asset.URL, tempPath, "runtime:download", progress); err != nil {
			return err
		}
		defer os.Remove(tempPath)

		log.Printf("[Downloader] Verifying hash for %s...", asset.Name)
		if err := verifyHash(tempPath, asset.Hash, asset.HashType); err != nil {
			return fmt.Errorf("hash verification failed for %s: %w", asset.Name, err)
		}

		log.Printf("[Downloader] Extracting %s to %s...", asset.Name, binDir)

		if strings.HasSuffix(asset.URL, ".zip") || strings.HasSuffix(asset.URL, "/package/") || strings.Contains(asset.URL, "nuget.org") {
			if err := unzipArchive(tempPath, binDir, asset.ExtractFiles); err != nil {
				return err
			}
		} else if strings.HasSuffix(asset.URL, ".tgz") || strings.HasSuffix(asset.URL, ".tar.gz") {
			f, err := os.Open(tempPath)
			if err != nil {
				return err
			}
			// Note: untarArchive extracts everything, we might want to filter like unzip
			if err := untarArchive(f, binDir); err != nil {
				f.Close()
				return err
			}
			f.Close()
		}
	}

	return nil
}

func verifyHash(filePath, expectedHash, hashType string) error {
	if expectedHash == "" {
		return nil
	}
	f, err := os.Open(filePath)
	if err != nil {
		return err
	}
	defer f.Close()

	var h io.Writer
	var hasher interface {
		Sum(b []byte) []byte
	}

	if hashType == "sha512" {
		s512 := sha512.New()
		h = s512
		hasher = s512
	} else {
		s256 := sha256.New()
		h = s256
		hasher = s256
	}

	if _, err := io.Copy(h, f); err != nil {
		return err
	}

	actualHash := hex.EncodeToString(hasher.Sum(nil))
	if actualHash != expectedHash {
		return fmt.Errorf("expected hash %s, got %s", expectedHash, actualHash)
	}
	return nil
}

func unzipArchive(src, dest string, filter []string) error {
	r, err := zip.OpenReader(src)
	if err != nil {
		return err
	}
	defer r.Close()

	for _, f := range r.File {
		shouldExtract := false
		if len(filter) == 0 {
			shouldExtract = true
		} else {
			for _, pattern := range filter {
				if f.Name == pattern || strings.HasSuffix(f.Name, "/"+pattern) {
					shouldExtract = true
					break
				}
			}
		}

		if !shouldExtract {
			continue
		}

		rc, err := f.Open()
		if err != nil {
			return err
		}

		// Flatten the path if it's a specific file we want (like DLLs)
		targetPath := filepath.Join(dest, filepath.Base(f.Name))
		if f.FileInfo().IsDir() {
			os.MkdirAll(targetPath, 0755)
		} else {
			os.MkdirAll(filepath.Dir(targetPath), 0755)
			outFile, err := os.OpenFile(targetPath, os.O_WRONLY|os.O_CREATE|os.O_TRUNC, f.Mode())
			if err != nil {
				rc.Close()
				return err
			}
			_, err = io.Copy(outFile, rc)
			outFile.Close()
			if err != nil {
				rc.Close()
				return err
			}
		}
		rc.Close()
	}
	return nil
}

// Downloader handles fetching embedding model files locally.
type Downloader struct{}

// NewDownloader creates a new Downloader instance.
func NewDownloader() *Downloader {
	return &Downloader{}
}

// EnsureLocal copies or downloads the model artifacts for the provided metadata.
// It returns an updated metadata struct with LocalPath + DownloadStatus fields filled in.
func (d *Downloader) EnsureLocal(meta *models.EmbeddingModelInfo, progress DownloadProgressCallback) (*models.EmbeddingModelInfo, error) {
	if meta == nil {
		return nil, fmt.Errorf("embedding model metadata cannot be nil")
	}
	if strings.TrimSpace(meta.ID) == "" {
		return nil, fmt.Errorf("embedding model must have an id")
	}

	if strings.EqualFold(meta.Backend, "fastembed") || strings.EqualFold(meta.SourceType, "fastembed") {
		return d.ensureFastEmbedModel(meta, progress)
	}

	sanitizedID := SanitizeModelID(meta.ID)
	modelsDir, err := utils.GetModelsDir()
	if err != nil {
		return nil, fmt.Errorf("failed to resolve models directory: %w", err)
	}

	targetDir := filepath.Join(modelsDir, sanitizedID)
	if err := os.MkdirAll(targetDir, 0755); err != nil {
		return nil, fmt.Errorf("failed to create model directory: %w", err)
	}

	targetPath := meta.LocalPath
	if targetPath == "" {
		resolved, err := ResolveModelPath(meta)
		if err != nil {
			return nil, err
		}
		targetPath = resolved
	}

	onnxReady := false
	stageModel := "model"
	if fileExists(targetPath) {
		onnxReady = true
		meta.LocalPath = targetPath
	} else if meta.SourceURI != "" {
		if err := d.retrieve(meta, meta.SourceURI, targetPath, stageModel, progress); err != nil {
			return nil, err
		}
		meta.LocalPath = targetPath
		onnxReady = true
	} else {
		return nil, fmt.Errorf("model %s has no usable source path for ONNX file", meta.ID)
	}

	tokenizerReady := meta.TokenizerURI == ""
	if meta.TokenizerURI != "" {
		tokenizerPath := meta.TokenizerLocalPath
		if tokenizerPath == "" {
			tokenizerPath = filepath.Join(targetDir, "tokenizer.json")
		}
		if !fileExists(tokenizerPath) {
			if err := d.retrieve(meta, meta.TokenizerURI, tokenizerPath, "tokenizer", progress); err != nil {
				return nil, fmt.Errorf("failed to download tokenizer for %s: %w", meta.ID, err)
			}
		}
		meta.TokenizerLocalPath = tokenizerPath
		tokenizerReady = true
	}

	if onnxReady && tokenizerReady {
		meta.DownloadStatus = "ready"
	} else if onnxReady {
		meta.DownloadStatus = "partial"
	} else {
		meta.DownloadStatus = "pending"
	}

	return meta, nil
}

func (d *Downloader) retrieve(meta *models.EmbeddingModelInfo, source, destination, stage string, progress DownloadProgressCallback) error {
	if strings.HasPrefix(source, "http://") || strings.HasPrefix(source, "https://") {
		return downloadFileWithProgress(meta.ID, source, destination, stage, progress)
	}
	// Treat as local file path
	return copyFileWithProgress(meta.ID, source, destination, stage, progress)
}
func fileExists(path string) bool {
	info, err := os.Stat(path)
	if err != nil {
		return false
	}
	return !info.IsDir()
}

func downloadFileWithProgress(modelID, url, destination, stage string, progress DownloadProgressCallback) error {
	resp, err := http.Get(url) // #nosec G107 -- user-provided URL expected
	if err != nil {
		return fmt.Errorf("failed to download %s: %w", url, err)
	}
	defer resp.Body.Close()

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return fmt.Errorf("failed to download %s: status %s", url, resp.Status)
	}

	out, err := os.Create(destination)
	if err != nil {
		return fmt.Errorf("failed to create %s: %w", destination, err)
	}

	total := resp.ContentLength
	var downloaded int64
	buf := make([]byte, 128*1024)
	for {
		n, readErr := resp.Body.Read(buf)
		if n > 0 {
			if _, writeErr := out.Write(buf[:n]); writeErr != nil {
				out.Close()
				os.Remove(destination)
				return writeErr
			}
			downloaded += int64(n)
			reportProgress(progress, modelID, stage, downloaded, total)
		}
		if readErr != nil {
			if readErr == io.EOF {
				break
			}
			out.Close()
			os.Remove(destination)
			return readErr
		}
	}
	out.Close()
	reportProgress(progress, modelID, stage, total, total)
	return nil
}

func copyFileWithProgress(modelID, source, destination, stage string, progress DownloadProgressCallback) error {
	in, err := os.Open(source)
	if err != nil {
		return fmt.Errorf("failed to open %s: %w", source, err)
	}
	defer in.Close()

	info, err := in.Stat()
	if err != nil {
		return fmt.Errorf("failed to stat %s: %w", source, err)
	}

	out, err := os.Create(destination)
	if err != nil {
		return fmt.Errorf("failed to create %s: %w", destination, err)
	}
	defer out.Close()

	var copied int64
	buf := make([]byte, 128*1024)
	for {
		n, readErr := in.Read(buf)
		if n > 0 {
			if _, writeErr := out.Write(buf[:n]); writeErr != nil {
				return writeErr
			}
			copied += int64(n)
			reportProgress(progress, modelID, stage, copied, info.Size())
		}
		if readErr != nil {
			if readErr == io.EOF {
				break
			}
			return readErr
		}
	}
	reportProgress(progress, modelID, stage, info.Size(), info.Size())
	return nil
}

func (d *Downloader) ensureFastEmbedModel(meta *models.EmbeddingModelInfo, progress DownloadProgressCallback) (*models.EmbeddingModelInfo, error) {
	modelID, err := mapFastEmbedModel(meta)
	if err != nil {
		return nil, err
	}
	cacheDir, err := fastEmbedCacheDir()
	if err != nil {
		return nil, err
	}
	targetDir := filepath.Join(cacheDir, string(modelID))
	if info, err := os.Stat(targetDir); err == nil && info.IsDir() {
		meta.LocalPath = targetDir
		meta.DownloadStatus = "ready"
		return meta, nil
	}
	if err := os.MkdirAll(cacheDir, 0755); err != nil {
		return nil, fmt.Errorf("failed to create fastembed cache dir: %w", err)
	}
	errDownload := downloadFastEmbedArchive(meta.ID, string(modelID), cacheDir, progress)
	if errDownload != nil {
		if base := fastEmbedHuggingFaceBase(meta); base != "" {
			if err := downloadFastEmbedFromHuggingFace(meta.ID, base, targetDir, progress); err != nil {
				return nil, fmt.Errorf("failed to download %s: %v (fallback failed: %v)", meta.ID, errDownload, err)
			}
		} else {
			return nil, errDownload
		}
	}
	meta.LocalPath = targetDir
	meta.DownloadStatus = "ready"
	return meta, nil
}

func downloadFastEmbedArchive(modelID, model string, cacheDir string, progress DownloadProgressCallback) error {
	url := fmt.Sprintf("https://storage.googleapis.com/qdrant-fastembed/%s.tar.gz", model)
	tempFile, err := os.CreateTemp(cacheDir, fmt.Sprintf("%s-*.tar.gz", model))
	if err != nil {
		return err
	}
	tempPath := tempFile.Name()
	tempFile.Close()
	if err := downloadFileWithProgress(modelID, url, tempPath, "fastembed:model", progress); err != nil {
		_ = os.Remove(tempPath)
		return err
	}
	defer os.Remove(tempPath)

	f, err := os.Open(tempPath)
	if err != nil {
		return err
	}
	defer f.Close()

	targetDir := filepath.Join(cacheDir, model)
	_ = os.RemoveAll(targetDir)

	if err := untarArchive(f, cacheDir); err != nil {
		return fmt.Errorf("failed to extract fastembed model %s: %w", model, err)
	}
	return nil
}

func untarArchive(r io.Reader, target string) error {
	gz, err := gzip.NewReader(r)
	if err != nil {
		return err
	}
	defer gz.Close()

	tr := tar.NewReader(gz)
	for {
		header, err := tr.Next()
		if err == io.EOF {
			break
		}
		if err != nil {
			return err
		}

		path := filepath.Join(target, header.Name)
		switch header.Typeflag {
		case tar.TypeDir:
			if err := os.MkdirAll(path, 0755); err != nil {
				return err
			}
		case tar.TypeReg:
			if err := os.MkdirAll(filepath.Dir(path), 0755); err != nil {
				return err
			}
			file, err := os.Create(path)
			if err != nil {
				return err
			}
			if _, err := io.Copy(file, tr); err != nil {
				file.Close()
				return err
			}
			file.Close()
		}
	}
	return nil
}

func downloadFastEmbedFromHuggingFace(modelID, base, targetDir string, progress DownloadProgressCallback) error {
	files := map[string]string{
		"config.json":               "config.json",
		"tokenizer.json":            "tokenizer.json",
		"tokenizer_config.json":     "tokenizer_config.json",
		"special_tokens_map.json":   "special_tokens_map.json",
		"vocab.txt":                 "vocab.txt",
		"sentence_bert_config.json": "sentence_bert_config.json",
		"model_optimized.onnx":      "onnx/model.onnx",
	}
	if err := os.RemoveAll(targetDir); err != nil && !os.IsNotExist(err) {
		return err
	}
	if err := os.MkdirAll(targetDir, 0755); err != nil {
		return err
	}
	for local, remote := range files {
		url := fmt.Sprintf("%s/%s", base, remote)
		stage := fmt.Sprintf("fastembed:%s", local)
		if err := downloadFileWithProgress(modelID, url, filepath.Join(targetDir, local), stage, progress); err != nil {
			return err
		}
	}
	return nil
}

func reportProgress(cb DownloadProgressCallback, modelID, stage string, downloaded, total int64) {
	if cb == nil {
		return
	}
	// log.Printf("[Downloader] Progress [%s]: %d/%d bytes", stage, downloaded, total)
	cb(DownloadProgress{
		ModelID:    modelID,
		Stage:      stage,
		Downloaded: downloaded,
		Total:      total,
	})
}
