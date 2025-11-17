/*
  File: vue_parser.go
  Purpose: Parser implementation for Vue.js Single File Components (.vue).
  Author: CodeTextor project
  Notes: Uses HTML parser to extract sections, then delegates to appropriate parsers.
*/

package chunker

import (
	"bytes"
	"regexp"
	"strings"

	sitter "github.com/tree-sitter/go-tree-sitter"
	tree_sitter_html "github.com/tree-sitter/tree-sitter-html/bindings/go"
)

// VueParser implements the LanguageParser interface for Vue.js SFC files.
// It extracts <template>, <script>, and <style> sections and parses each appropriately.
type VueParser struct {
	htmlParser *HTMLParser
	jsParser   *TypeScriptParser
	cssParser  *CSSParser
}

// sectionInfo holds information about a Vue SFC section
type sectionInfo struct {
	name         string
	content      []byte
	startLine    uint32
	endLine      uint32
	startByte    uint32
	endByte      uint32
	isTypeScript bool
}

// GetLanguage returns the tree-sitter Language for HTML (used for structure).
func (v *VueParser) GetLanguage() *sitter.Language {
	return sitter.NewLanguage(tree_sitter_html.Language())
}

// GetFileExtensions returns the file extensions handled by this parser.
func (v *VueParser) GetFileExtensions() []string {
	return []string{".vue"}
}

// ExtractSymbols extracts symbols from a Vue SFC file.
// It parses:
//   - Template section (HTML elements)
//   - Script section (JavaScript/TypeScript code)
//   - Style section (CSS rules)
func (v *VueParser) ExtractSymbols(tree *sitter.Tree, source []byte) ([]Symbol, error) {
	var symbols []Symbol

	// Initialize sub-parsers if needed
	if v.htmlParser == nil {
		v.htmlParser = &HTMLParser{}
	}
	if v.jsParser == nil {
		v.jsParser = &TypeScriptParser{isTypeScript: false}
	}
	if v.cssParser == nil {
		v.cssParser = &CSSParser{}
	}

	// Extract sections with position information
	sections := v.extractSectionsWithPosition(source)

	// Parse template section
	if templateSection, ok := sections["template"]; ok {
		templateSymbol := Symbol{
			Name:       "template",
			Kind:       SymbolElement,
			StartLine:  templateSection.startLine,
			EndLine:    templateSection.endLine,
			StartByte:  templateSection.startByte,
			EndByte:    templateSection.endByte,
			Source:     string(source[templateSection.startByte:templateSection.endByte]),
			Visibility: "public",
		}
		symbols = append(symbols, templateSymbol)

		templateSymbols, err := v.parseHTMLSection(templateSection, "template")
		if err == nil {
			symbols = append(symbols, templateSymbols...)
		}
	}

	// Parse script section
	if scriptSection, ok := sections["script"]; ok {
		scriptSymbol := Symbol{
			Name:       "script",
			Kind:       SymbolScript,
			StartLine:  scriptSection.startLine,
			EndLine:    scriptSection.endLine,
			StartByte:  scriptSection.startByte,
			EndByte:    scriptSection.endByte,
			Source:     string(source[scriptSection.startByte:scriptSection.endByte]),
			Visibility: "public",
		}
		symbols = append(symbols, scriptSymbol)

		scriptSymbols, err := v.parseScriptSection(scriptSection, "script")
		if err == nil {
			symbols = append(symbols, scriptSymbols...)
		}
	}

	// Parse style section
	if styleSection, ok := sections["style"]; ok {
		styleSymbol := Symbol{
			Name:       "style",
			Kind:       SymbolStyle,
			StartLine:  styleSection.startLine,
			EndLine:    styleSection.endLine,
			StartByte:  styleSection.startByte,
			EndByte:    styleSection.endByte,
			Source:     string(source[styleSection.startByte:styleSection.endByte]),
			Visibility: "public",
		}
		symbols = append(symbols, styleSymbol)

		styleSymbols, err := v.parseStyleSection(styleSection, "style")
		if err == nil {
			symbols = append(symbols, styleSymbols...)
		}
	}

	return symbols, nil
}

// extractSectionsWithPosition extracts <template>, <script>, and <style> sections from Vue SFC
// with their positions in the original file.
func (v *VueParser) extractSectionsWithPosition(source []byte) map[string]sectionInfo {
	sections := make(map[string]sectionInfo)

	// Regular expressions to match Vue SFC sections
	templateRe := regexp.MustCompile(`(?s)<template[^>]*>(.*?)</template>`)
	scriptRe := regexp.MustCompile(`(?s)<script([^>]*)>(.*?)</script>`)
	styleRe := regexp.MustCompile(`(?s)<style[^>]*>(.*?)</style>`)

	// Extract template with position
	if match := templateRe.FindSubmatchIndex(source); match != nil && len(match) >= 4 {
		contentStart := match[2]
		contentEnd := match[3]
		content := source[contentStart:contentEnd]
		content = bytes.TrimSpace(content)

		sections["template"] = sectionInfo{
			name:      "template",
			content:   content,
			startLine: v.getLineNumber(source, match[0]),
			endLine:   v.getLineNumber(source, match[1]),
			startByte: uint32(match[0]),
			endByte:   uint32(match[1]),
		}
	}

	// Extract script with position
	if match := scriptRe.FindSubmatchIndex(source); match != nil && len(match) >= 6 {
		attrStart := match[2]
		attrEnd := match[3]
		contentStart := match[4]
		contentEnd := match[5]
		content := source[contentStart:contentEnd]
		content = bytes.TrimSpace(content)
		attrs := strings.ToLower(string(source[attrStart:attrEnd]))
		isTS := strings.Contains(attrs, "lang=\"ts\"") ||
			strings.Contains(attrs, "lang='ts'") ||
			strings.Contains(attrs, "lang=\"tsx\"") ||
			strings.Contains(attrs, "lang='tsx'") ||
			strings.Contains(attrs, "lang=\"typescript\"") ||
			strings.Contains(attrs, "lang='typescript'")

		sections["script"] = sectionInfo{
			name:         "script",
			content:      content,
			startLine:    v.getLineNumber(source, match[0]),
			endLine:      v.getLineNumber(source, match[1]),
			startByte:    uint32(match[0]),
			endByte:      uint32(match[1]),
			isTypeScript: isTS,
		}
	}

	// Extract style with position
	if match := styleRe.FindSubmatchIndex(source); match != nil && len(match) >= 4 {
		contentStart := match[2]
		contentEnd := match[3]
		content := source[contentStart:contentEnd]
		content = bytes.TrimSpace(content)

		sections["style"] = sectionInfo{
			name:      "style",
			content:   content,
			startLine: v.getLineNumber(source, match[0]),
			endLine:   v.getLineNumber(source, match[1]),
			startByte: uint32(match[0]),
			endByte:   uint32(match[1]),
		}
	}

	return sections
}

// getLineNumber calculates the line number (1-indexed) for a given byte position.
func (v *VueParser) getLineNumber(source []byte, bytePos int) uint32 {
	line := uint32(1)
	for i := 0; i < bytePos && i < len(source); i++ {
		if source[i] == '\n' {
			line++
		}
	}
	return line
}

// parseHTMLSection parses the template section using HTML parser.
func (v *VueParser) parseHTMLSection(section sectionInfo, sectionName string) ([]Symbol, error) {
	// Create a temporary parser for HTML
	htmlParser := sitter.NewParser()
	defer htmlParser.Close()

	err := htmlParser.SetLanguage(v.htmlParser.GetLanguage())
	if err != nil {
		return nil, err
	}

	tree := htmlParser.Parse(section.content, nil)
	if tree == nil {
		return nil, nil
	}
	defer tree.Close()

	symbols, err := v.htmlParser.ExtractSymbols(tree, section.content)
	if err != nil {
		return nil, err
	}

	// Calculate line offset for this section
	// The content starts at the line after the opening tag
	lineOffset := section.startLine

	// Adjust line numbers and set parent for root-level elements only
	for i := range symbols {
		symbols[i].StartLine += lineOffset
		symbols[i].EndLine += lineOffset
		symbols[i].StartByte += section.startByte
		symbols[i].EndByte += section.startByte

		// Only set the section as parent for root-level elements (those without a parent)
		// This preserves the HTML hierarchy within the template
		if symbols[i].Parent == "" {
			symbols[i].Parent = sectionName
		}
		// Elements with parents keep their original hierarchy
	}

	return symbols, nil
}

// parseScriptSection parses the script section using JavaScript/TypeScript parser.
func (v *VueParser) parseScriptSection(section sectionInfo, sectionName string) ([]Symbol, error) {
	// Create a temporary parser for JavaScript
	jsParser := sitter.NewParser()
	defer jsParser.Close()

	parser := &TypeScriptParser{isTypeScript: section.isTypeScript}
	err := jsParser.SetLanguage(parser.GetLanguage())
	if err != nil {
		return nil, err
	}

	tree := jsParser.Parse(section.content, nil)
	if tree == nil {
		return nil, nil
	}
	defer tree.Close()

	symbols, err := parser.ExtractSymbols(tree, section.content)
	if err != nil {
		return nil, err
	}

	// Calculate line offset for this section
	lineOffset := section.startLine

	// Adjust line numbers and set parent for root-level symbols only
	for i := range symbols {
		symbols[i].StartLine += lineOffset
		symbols[i].EndLine += lineOffset
		symbols[i].StartByte += section.startByte
		symbols[i].EndByte += section.startByte

		// Only set the section as parent for root-level symbols (those without a parent)
		// This preserves the JavaScript/TypeScript hierarchy within the script
		if symbols[i].Parent == "" {
			symbols[i].Parent = sectionName
		}
		// Symbols with parents keep their original hierarchy
	}

	return symbols, nil
}

// parseStyleSection parses the style section using CSS parser.
func (v *VueParser) parseStyleSection(section sectionInfo, sectionName string) ([]Symbol, error) {
	// Create a temporary parser for CSS
	cssParser := sitter.NewParser()
	defer cssParser.Close()

	err := cssParser.SetLanguage(v.cssParser.GetLanguage())
	if err != nil {
		return nil, err
	}

	tree := cssParser.Parse(section.content, nil)
	if tree == nil {
		return nil, nil
	}
	defer tree.Close()

	symbols, err := v.cssParser.ExtractSymbols(tree, section.content)
	if err != nil {
		return nil, err
	}

	// Calculate line offset for this section
	lineOffset := section.startLine

	// Adjust line numbers and set parent for root-level rules only
	for i := range symbols {
		symbols[i].StartLine += lineOffset
		symbols[i].EndLine += lineOffset
		symbols[i].StartByte += section.startByte
		symbols[i].EndByte += section.startByte

		// Only set the section as parent for root-level rules (those without a parent)
		// This preserves any CSS hierarchy (e.g., nested rules in SCSS/LESS)
		if symbols[i].Parent == "" {
			symbols[i].Parent = sectionName
		}
		// Rules with parents keep their original hierarchy
	}

	return symbols, nil
}

// ExtractImports extracts imports from all sections of a Vue SFC.
func (v *VueParser) ExtractImports(tree *sitter.Tree, source []byte) ([]string, error) {
	var imports []string

	// Initialize sub-parsers if needed
	if v.htmlParser == nil {
		v.htmlParser = &HTMLParser{}
	}
	if v.jsParser == nil {
		v.jsParser = &TypeScriptParser{isTypeScript: false}
	}
	if v.cssParser == nil {
		v.cssParser = &CSSParser{}
	}

	// Extract sections
	sections := v.extractSectionsWithPosition(source)

	// Extract imports from script section
	if scriptSection, ok := sections["script"]; ok {
		jsParser := sitter.NewParser()
		defer jsParser.Close()

		parser := &TypeScriptParser{isTypeScript: scriptSection.isTypeScript}
		err := jsParser.SetLanguage(parser.GetLanguage())
		if err == nil {
			scriptTree := jsParser.Parse(scriptSection.content, nil)
			if scriptTree != nil {
				defer scriptTree.Close()
				scriptImports, _ := parser.ExtractImports(scriptTree, scriptSection.content)
				imports = append(imports, scriptImports...)
			}
		}
	}

	// Extract imports from style section (@import rules)
	if styleSection, ok := sections["style"]; ok {
		cssParser := sitter.NewParser()
		defer cssParser.Close()

		err := cssParser.SetLanguage(v.cssParser.GetLanguage())
		if err == nil {
			styleTree := cssParser.Parse(styleSection.content, nil)
			if styleTree != nil {
				defer styleTree.Close()
				styleImports, _ := v.cssParser.ExtractImports(styleTree, styleSection.content)
				imports = append(imports, styleImports...)
			}
		}
	}

	return imports, nil
}
