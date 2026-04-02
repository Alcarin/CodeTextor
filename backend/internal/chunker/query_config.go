package chunker

import (
	"embed"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"

	"github.com/BurntSushi/toml"
)

// LanguageConfig holds the full configuration for a language parser.
type LanguageConfig struct {
	Language LanguageInfo  `toml:"language"`
	Queries  QueriesConfig `toml:"queries"`
	Rules    RulesConfig   `toml:"rules"`
}

// LanguageInfo contains general information about the language.
type LanguageInfo struct {
	Name       string   `toml:"name"`
	Extensions []string `toml:"extensions"`
	Grammar    string   `toml:"grammar"`
}

// QueriesConfig contains the Tree-sitter queries for symbol extraction.
type QueriesConfig struct {
	Symbols  string            `toml:"symbols"`
	Imports  string            `toml:"imports"`
	Metadata string            `toml:"metadata"`
	Usages   string            `toml:"usages"`
	Extra    map[string]string `toml:"extra"`
}

// RulesConfig defines behavioral rules for the parser engine.
type RulesConfig struct {
	CommentPrefixes []string       `toml:"comment_prefixes"`
	Visibility      VisibilityRule `toml:"visibility"`
	Todo            TodoRule       `toml:"todo"`
}

// VisibilityRule defines how to determine symbol visibility.
type VisibilityRule struct {
	Type string `toml:"type"` // "first_letter_case", "prefix_underscore", "keyword", "public"
}

// TodoRule defines how to find TODO comments.
type TodoRule struct {
	Pattern   string   `toml:"pattern"`
	NodeTypes []string `toml:"node_types"`
}

// LoadConfigFromBytes parses TOML data into a LanguageConfig.
func LoadConfigFromBytes(data []byte) (*LanguageConfig, error) {
	var config LanguageConfig
	_, err := toml.Decode(string(data), &config)
	if err != nil {
		return nil, fmt.Errorf("failed to decode TOML: %w", err)
	}
	return &config, nil
}

// LoadDefaultConfigs loads all TOML configurations embedded in the binary.
func LoadDefaultConfigs(embeddedFS embed.FS) (map[string]*LanguageConfig, error) {
	configs := make(map[string]*LanguageConfig)

	err := fs.WalkDir(embeddedFS, "parsers/default", func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() || filepath.Ext(path) != ".toml" {
			return nil
		}

		data, err := embeddedFS.ReadFile(path)
		if err != nil {
			return fmt.Errorf("failed to read embedded file %s: %w", path, err)
		}

		config, err := LoadConfigFromBytes(data)
		if err != nil {
			return fmt.Errorf("failed to parse embedded config %s: %w", path, err)
		}

		configs[config.Language.Name] = config
		return nil
	})

	if err != nil {
		return nil, err
	}

	return configs, nil
}

// LoadUserConfigs loads TOML configurations from a user-defined directory.
func LoadUserConfigs(dir string) (map[string]*LanguageConfig, error) {
	configs := make(map[string]*LanguageConfig)

	if _, err := os.Stat(dir); os.IsNotExist(err) {
		return configs, nil // No user configs, not an error
	}

	files, err := os.ReadDir(dir)
	if err != nil {
		return nil, fmt.Errorf("failed to read user config dir: %w", err)
	}

	for _, file := range files {
		if file.IsDir() || filepath.Ext(file.Name()) != ".toml" {
			continue
		}

		path := filepath.Join(dir, file.Name())
		data, err := os.ReadFile(path)
		if err != nil {
			return nil, fmt.Errorf("failed to read user config %s: %w", path, err)
		}

		config, err := LoadConfigFromBytes(data)
		if err != nil {
			return nil, fmt.Errorf("failed to parse user config %s: %w", path, err)
		}

		configs[config.Language.Name] = config
	}

	return configs, nil
}

// MergeConfigs merges default and user configurations. User configurations
// override default ones with the same language name.
func MergeConfigs(defaults, user map[string]*LanguageConfig) map[string]*LanguageConfig {
	merged := make(map[string]*LanguageConfig)

	// Start with defaults
	for name, config := range defaults {
		merged[name] = config
	}

	// Override with user configs
	for name, config := range user {
		merged[name] = config
	}

	return merged
}
