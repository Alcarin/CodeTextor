/*
  File: paths.go
  Purpose: Cross-platform path utilities for CodeTextor.
  Author: CodeTextor project
  Notes: Provides OS-independent path handling for databases and configuration.
*/

package utils

import (
	"os"
	"path/filepath"
)

// GetAppDataDir returns the application data directory for CodeTextor.
// This directory is OS-specific:
//   - Linux: ~/.local/share/codetextor
//   - macOS: ~/Library/Application Support/codetextor
//   - Windows: %LOCALAPPDATA%/codetextor
//
// The directory is created if it doesn't exist.
// Returns an error if the directory cannot be created.
func GetAppDataDir() (string, error) {
	var baseDir string

	// Get the user's home directory
	homeDir, err := os.UserHomeDir()
	if err != nil {
		return "", err
	}

	// Determine OS-specific data directory
	switch {
	case isLinux():
		baseDir = filepath.Join(homeDir, ".local", "share", "codetextor")
	case isDarwin():
		baseDir = filepath.Join(homeDir, "Library", "Application Support", "codetextor")
	case isWindows():
		// On Windows, prefer LOCALAPPDATA if available
		appData := os.Getenv("LOCALAPPDATA")
		if appData != "" {
			baseDir = filepath.Join(appData, "codetextor")
		} else {
			baseDir = filepath.Join(homeDir, "AppData", "Local", "codetextor")
		}
	default:
		// Fallback for unknown OS
		baseDir = filepath.Join(homeDir, ".codetextor")
	}

	// Create directory if it doesn't exist
	if err := os.MkdirAll(baseDir, 0755); err != nil {
		return "", err
	}

	return baseDir, nil
}

// GetIndexesDir returns the directory where project index databases are stored.
// Returns: <AppDataDir>/indexes/
// Creates the directory if it doesn't exist.
func GetIndexesDir() (string, error) {
	appDir, err := GetAppDataDir()
	if err != nil {
		return "", err
	}

	indexesDir := filepath.Join(appDir, "indexes")
	if err := os.MkdirAll(indexesDir, 0755); err != nil {
		return "", err
	}

	return indexesDir, nil
}

// GetConfigDir returns the directory where configuration files are stored.
// Returns: <AppDataDir>/config/
// Creates the directory if it doesn't exist.
func GetConfigDir() (string, error) {
	appDir, err := GetAppDataDir()
	if err != nil {
		return "", err
	}

	configDir := filepath.Join(appDir, "config")
	if err := os.MkdirAll(configDir, 0755); err != nil {
		return "", err
	}

	return configDir, nil
}

// GetModelsDir returns the directory where embedding models are stored.
// Returns: <AppDataDir>/models/
func GetModelsDir() (string, error) {
	appDir, err := GetAppDataDir()
	if err != nil {
		return "", err
	}

	modelsDir := filepath.Join(appDir, "models")
	if err := os.MkdirAll(modelsDir, 0755); err != nil {
		return "", err
	}

	return modelsDir, nil
}

// GetProjectDBPath returns the full path to a project's index database file.
// Parameters:
//   - projectID: the unique project identifier
//
// Returns: <IndexesDir>/<projectID>.db
func GetProjectDBPath(projectID string) (string, error) {
	indexesDir, err := GetIndexesDir()
	if err != nil {
		return "", err
	}

	return filepath.Join(indexesDir, projectID+".db"), nil
}

// GetProjectsConfigPath returns the path to the projects configuration file.
// Returns: <ConfigDir>/projects.json
func GetProjectsConfigPath() (string, error) {
	configDir, err := GetConfigDir()
	if err != nil {
		return "", err
	}

	return filepath.Join(configDir, "projects.json"), nil
}

// isLinux checks if the current OS is Linux.
func isLinux() bool {
	return os.PathSeparator == '/' && fileExists("/etc") && !fileExists("/System/Library")
}

// isDarwin checks if the current OS is macOS.
func isDarwin() bool {
	return os.PathSeparator == '/' && fileExists("/System/Library")
}

// isWindows checks if the current OS is Windows.
func isWindows() bool {
	return os.PathSeparator == '\\' || filepath.Separator == '\\'
}

// fileExists checks if a file or directory exists.
func fileExists(path string) bool {
	_, err := os.Stat(path)
	return err == nil
}
