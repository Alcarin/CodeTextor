package embedding

import (
	"CodeTextor/backend/pkg/models"
	"errors"
	"fmt"
	"log"
	"math"
	"os"
	"strings"
	"sync"

	"github.com/sugarme/tokenizer"
	"github.com/sugarme/tokenizer/pretrained"
	onnx "github.com/yalue/onnxruntime_go"
)

var (
	onnxRuntimeInitOnce     sync.Once
	onnxRuntimeInitErr      error
	onnxSharedLibraryPath   string
	activeSharedLibraryPath string
)

// ONNXEmbeddingClient uses ONNX Runtime + HuggingFace tokenizer.json files to compute embeddings.
type ONNXEmbeddingClient struct {
	session          *onnx.DynamicAdvancedSession
	tokenizer        *tokenizer.Tokenizer
	padID            int
	padTypeID        int
	padToken         string
	padDirection     tokenizer.PaddingDirection
	maxSeqLen        int
	inputNames       []string
	outputNames      []string
	expectTokenTypes bool
	dimension        int
	mu               sync.Mutex
}

// NewONNXEmbeddingClient constructs an embedding client backed by an ONNX model.
func NewONNXEmbeddingClient(meta *models.EmbeddingModelInfo) (*ONNXEmbeddingClient, error) {
	if meta == nil {
		return nil, fmt.Errorf("embedding metadata is required")
	}
	if strings.TrimSpace(meta.LocalPath) == "" {
		return nil, fmt.Errorf("model %s is missing a local ONNX path", meta.ID)
	}
	if strings.TrimSpace(meta.TokenizerLocalPath) == "" {
		return nil, fmt.Errorf("model %s is missing a tokenizer.json path", meta.ID)
	}
	if err := ensureONNXRuntimeInitialized(); err != nil {
		log.Printf("DEBUG: ensureONNXRuntimeInitialized failed: %v", err)
		return nil, err
	}
	log.Printf("DEBUG: ONNX Runtime initialized successfully")

	tk, err := pretrained.FromFile(meta.TokenizerLocalPath)
	if err != nil {
		return nil, fmt.Errorf("failed to load tokenizer for %s: %w", meta.ID, err)
	}

	maxSeq := meta.MaxSequenceLength
	if maxSeq <= 0 {
		maxSeq = 512
	}
	padParams := tk.GetPadding()
	padID := 0
	padType := 0
	padToken := "[PAD]"
	padDirection := tokenizer.Right
	if padParams != nil {
		padID = padParams.PadId
		padType = padParams.PadTypeId
		if padParams.PadToken != "" {
			padToken = padParams.PadToken
		}
		padDirection = padParams.Direction
	}

	inputInfo, outputInfo, err := onnx.GetInputOutputInfo(meta.LocalPath)
	if err != nil {
		return nil, fmt.Errorf("failed to inspect ONNX file %s: %w", meta.LocalPath, err)
	}
	if len(inputInfo) == 0 || len(outputInfo) == 0 {
		return nil, fmt.Errorf("ONNX model %s is missing inputs or outputs", meta.LocalPath)
	}

	inputNames := make([]string, len(inputInfo))
	for i, info := range inputInfo {
		inputNames[i] = info.Name
	}
	outputNames := make([]string, len(outputInfo))
	for i, info := range outputInfo {
		outputNames[i] = info.Name
	}

	session, err := newONNXSessionWithOptionalCUDA(meta.LocalPath, inputNames, outputNames)
	if err != nil {
		log.Printf("DEBUG: newONNXSessionWithOptionalCUDA failed: %v", err)
		return nil, fmt.Errorf("failed to create ONNX session: %w", err)
	}
	log.Printf("DEBUG: ONNX session created successfully for %s", meta.LocalPath)

	client := &ONNXEmbeddingClient{
		session:          session,
		tokenizer:        tk,
		padID:            padID,
		padTypeID:        padType,
		padToken:         padToken,
		padDirection:     padDirection,
		maxSeqLen:        maxSeq,
		inputNames:       inputNames,
		outputNames:      outputNames,
		expectTokenTypes: hasTokenTypeInput(inputNames),
		dimension:        meta.Dimension,
	}
	return client, nil
}

// GenerateEmbeddings converts each input string into a normalized embedding vector.
func (c *ONNXEmbeddingClient) GenerateEmbeddings(texts []string) ([][]float32, error) {
	c.mu.Lock()
	defer c.mu.Unlock()

	results := make([][]float32, len(texts))
	for i, text := range texts {
		vec, err := c.embedSingle(text)
		if err != nil {
			return nil, err
		}
		results[i] = vec
	}
	return results, nil
}

// Close releases ONNX runtime resources.
func (c *ONNXEmbeddingClient) Close() error {
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.session != nil {
		if err := c.session.Destroy(); err != nil {
			return err
		}
		c.session = nil
	}
	return nil
}

func (c *ONNXEmbeddingClient) embedSingle(text string) ([]float32, error) {
	encoding, err := c.tokenizer.EncodeSingle(text, true)
	if err != nil {
		return nil, fmt.Errorf("failed to encode text: %w", err)
	}
	if encoding == nil {
		return nil, errors.New("tokenizer returned nil encoding")
	}

	// Older versions of sugarme/tokenizer sometimes return encodings whose
	// auxiliary slices (Words, TypeIds, etc.) are shorter than Ids, which makes
	// Truncate panic when asked to keep more items than they contain. Bring all
	// slices to the same length before truncating/padding to avoid that crash.
	c.normalizeEncoding(encoding)

	if encoding.Len() > c.maxSeqLen {
		truncated, err := encoding.Truncate(c.maxSeqLen, 0)
		if err != nil {
			return nil, fmt.Errorf("failed to truncate encoding: %w", err)
		}
		encoding = truncated
	}
	if encoding.Len() < c.maxSeqLen {
		encoding = encoding.Pad(c.maxSeqLen, c.padID, c.padTypeID, c.padToken, c.padDirection)
	}

	ids := encoding.GetIds()
	attMask := encoding.GetAttentionMask()
	if len(ids) > c.maxSeqLen {
		ids = ids[:c.maxSeqLen]
	}
	if len(attMask) > c.maxSeqLen {
		attMask = attMask[:c.maxSeqLen]
	}

	tokenTypeIDs := make([]int, c.maxSeqLen)
	if c.expectTokenTypes {
		typeIDs := encoding.GetTypeIds()
		copy(tokenTypeIDs, clampSlice(typeIDs, c.maxSeqLen))
	}

	inputTensors, cleanupInputs, err := c.buildInputTensors(ids, attMask, tokenTypeIDs)
	if err != nil {
		return nil, err
	}
	defer cleanupInputs()

	outputValues := make([]onnx.Value, len(c.outputNames))
	err = c.session.Run(inputTensors, outputValues)
	defer func() {
		for _, out := range outputValues {
			if out != nil {
				out.Destroy()
			}
		}
	}()
	if err != nil {
		return nil, fmt.Errorf("failed to run ONNX session: %w", err)
	}

	if len(outputValues) == 0 {
		return nil, fmt.Errorf("model returned no outputs")
	}

	tensor, ok := outputValues[0].(*onnx.Tensor[float32])
	if !ok {
		return nil, fmt.Errorf("unexpected output tensor type %T", outputValues[0])
	}

	vec, err := c.postProcessEmbedding(tensor.GetData(), tensor.GetShape(), attMask)
	if err != nil {
		return nil, err
	}
	normalizeVector(vec)
	return vec, nil
}

func (c *ONNXEmbeddingClient) buildInputTensors(ids []int, attMask []int, tokenTypeIDs []int) ([]onnx.Value, func(), error) {
	shape := onnx.NewShape(1, int64(c.maxSeqLen))
	idTensor, err := onnx.NewTensor(shape, toInt64(ids, c.maxSeqLen))
	if err != nil {
		return nil, nil, fmt.Errorf("failed to build input_ids tensor: %w", err)
	}
	attTensor, err := onnx.NewTensor(shape, toInt64(attMask, c.maxSeqLen))
	if err != nil {
		idTensor.Destroy()
		return nil, nil, fmt.Errorf("failed to build attention_mask tensor: %w", err)
	}
	var tokenTensor *onnx.Tensor[int64]
	if c.expectTokenTypes {
		tokenTensor, err = onnx.NewTensor(shape, toInt64(tokenTypeIDs, c.maxSeqLen))
		if err != nil {
			idTensor.Destroy()
			attTensor.Destroy()
			return nil, nil, fmt.Errorf("failed to build token_type_ids tensor: %w", err)
		}
	}

	cleanup := func() {
		idTensor.Destroy()
		attTensor.Destroy()
		if tokenTensor != nil {
			tokenTensor.Destroy()
		}
	}

	values := make([]onnx.Value, 0, len(c.inputNames))
	for _, name := range c.inputNames {
		switch strings.ToLower(name) {
		case "input_ids":
			values = append(values, idTensor)
		case "attention_mask":
			values = append(values, attTensor)
		case "token_type_ids":
			if tokenTensor == nil {
				return nil, cleanup, fmt.Errorf("model expects token_type_ids but tokenizer did not provide them")
			}
			values = append(values, tokenTensor)
		default:
			return nil, cleanup, fmt.Errorf("unsupported ONNX input %s", name)
		}
	}
	return values, cleanup, nil
}

func (c *ONNXEmbeddingClient) postProcessEmbedding(data []float32, shape onnx.Shape, attMask []int) ([]float32, error) {
	if len(shape) == 2 {
		// shape: [1, hidden]
		vec := append([]float32(nil), data...)
		return vec, nil
	}
	if len(shape) != 3 {
		return nil, fmt.Errorf("unsupported output shape %v", shape)
	}
	if len(shape) < 3 {
		return nil, fmt.Errorf("invalid hidden state shape %v", shape)
	}
	seqLen := int(shape[1])
	hidden := int(shape[2])
	if seqLen <= 0 || hidden <= 0 {
		return nil, fmt.Errorf("invalid output dimensions %v", shape)
	}
	if len(data) != seqLen*hidden {
		return nil, fmt.Errorf("mismatched output size: got %d expected %d", len(data), seqLen*hidden)
	}

	result := make([]float32, hidden)
	var count float32
	for i := 0; i < seqLen && i < len(attMask); i++ {
		if attMask[i] == 0 {
			continue
		}
		start := i * hidden
		for j := 0; j < hidden; j++ {
			result[j] += data[start+j]
		}
		count++
	}
	if count == 0 {
		count = 1
	}
	scale := 1 / count
	for i := range result {
		result[i] *= scale
	}
	return result, nil
}

func newONNXSessionWithOptionalCUDA(modelPath string, inputNames, outputNames []string) (*onnx.DynamicAdvancedSession, error) {
	return onnx.NewDynamicAdvancedSession(modelPath, inputNames, outputNames, nil)
}

func ensureONNXRuntimeInitialized() error {
	onnxRuntimeInitOnce.Do(func() {
		if trimmed := strings.TrimSpace(onnxSharedLibraryPath); trimmed != "" {
			onnx.SetSharedLibraryPath(trimmed)
		}
		onnxRuntimeInitErr = onnx.InitializeEnvironment()
		if onnxRuntimeInitErr == nil {
			activeSharedLibraryPath = onnxSharedLibraryPath
		}
	})
	return onnxRuntimeInitErr
}

// ConfigureSharedLibraryPath sets the desired ONNX Runtime shared library path to be used on initialization.
func ConfigureSharedLibraryPath(path string) {
	onnxSharedLibraryPath = strings.TrimSpace(path)
}

// ActiveSharedLibraryPath returns the ONNX Runtime shared library path currently in use.
func ActiveSharedLibraryPath() string {
	return strings.TrimSpace(activeSharedLibraryPath)
}

// IsONNXRuntimeInstalled checks if the shared library exists without loading it.
func IsONNXRuntimeInstalled() bool {
	path := strings.TrimSpace(onnxSharedLibraryPath)
	if path != "" {
		if info, err := os.Stat(path); err == nil && !info.IsDir() {
			return true
		}
		return false
	}

	available, err := CheckONNXRuntimeAvailability()
	return err == nil && available
}

// CheckONNXRuntimeAvailability reports whether the ONNX runtime shared library
// can be loaded in the current process. The detection result is memoized, so
// subsequent calls return immediately without touching the runtime again.
func CheckONNXRuntimeAvailability() (bool, error) {
	if err := ensureONNXRuntimeInitialized(); err != nil {
		return false, err
	}
	return true, nil
}

func hasTokenTypeInput(inputNames []string) bool {
	for _, name := range inputNames {
		if strings.EqualFold(name, "token_type_ids") {
			return true
		}
	}
	return false
}

func toInt64(values []int, maxLen int) []int64 {
	out := make([]int64, maxLen)
	for i := 0; i < maxLen && i < len(values); i++ {
		out[i] = int64(values[i])
	}
	return out
}

func clampSlice(values []int, maxLen int) []int {
	if len(values) >= maxLen {
		return values[:maxLen]
	}
	out := make([]int, maxLen)
	copy(out, values)
	return out
}

func normalizeVector(vec []float32) {
	var sum float64
	for _, v := range vec {
		sum += float64(v * v)
	}
	if sum == 0 {
		return
	}
	norm := float32(1 / math.Sqrt(sum))
	for i := range vec {
		vec[i] *= norm
	}
}

func (c *ONNXEmbeddingClient) normalizeEncoding(enc *tokenizer.Encoding) {
	if enc == nil {
		return
	}
	targetLen := len(enc.Ids)
	if targetLen == 0 {
		return
	}

	enc.TypeIds = padOrTrimInt(enc.TypeIds, targetLen, c.padTypeID)
	enc.Tokens = padOrTrimString(enc.Tokens, targetLen, c.padToken)
	enc.SpecialTokenMask = padOrTrimInt(enc.SpecialTokenMask, targetLen, 0)
	enc.AttentionMask = padOrTrimInt(enc.AttentionMask, targetLen, 1)
	enc.Offsets = padOrTrimOffsets(enc.Offsets, targetLen)
	enc.Words = padOrTrimInt(enc.Words, targetLen, -1)
}

func padOrTrimInt(values []int, targetLen int, fill int) []int {
	if len(values) == targetLen {
		return values
	}
	out := make([]int, targetLen)
	if len(values) > targetLen {
		copy(out, values[:targetLen])
		return out
	}
	copy(out, values)
	for i := len(values); i < targetLen; i++ {
		out[i] = fill
	}
	return out
}

func padOrTrimString(values []string, targetLen int, fill string) []string {
	if len(values) == targetLen {
		return values
	}
	out := make([]string, targetLen)
	if len(values) > targetLen {
		copy(out, values[:targetLen])
		return out
	}
	copy(out, values)
	for i := len(values); i < targetLen; i++ {
		out[i] = fill
	}
	return out
}

func padOrTrimOffsets(values [][]int, targetLen int) [][]int {
	if len(values) == targetLen {
		return values
	}
	out := make([][]int, targetLen)
	if len(values) > targetLen {
		copy(out, values[:targetLen])
		return out
	}
	copy(out, values)
	for i := len(values); i < targetLen; i++ {
		out[i] = []int{0, 0}
	}
	return out
}
