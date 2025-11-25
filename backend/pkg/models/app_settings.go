package models

// ONNXRuntimeSettings describes the runtime configuration persisted for ONNX.
type ONNXRuntimeSettings struct {
	// SharedLibraryPath is the user-provided location of the ONNX shared library.
	SharedLibraryPath string `json:"sharedLibraryPath"`
	// ActivePath is the path currently used by the initialized runtime (if any).
	ActivePath string `json:"activePath,omitempty"`
	// RuntimeAvailable reports whether the runtime was successfully initialized at startup.
	RuntimeAvailable bool `json:"runtimeAvailable"`
	// RequiresRestart indicates that the saved path differs from the active runtime and
	// a restart is needed for the change to take effect.
	RequiresRestart bool `json:"requiresRestart"`
}

// ONNXRuntimeTestResult captures a quick validation of a user-provided ONNX path.
type ONNXRuntimeTestResult struct {
	Success bool   `json:"success"`
	Message string `json:"message"`
	Error   string `json:"error,omitempty"`
}
