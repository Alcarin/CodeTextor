/*
  File: vue_parser.go
  Purpose: Parser implementation for Vue.js Single File Components (.vue).
  Author: CodeTextor project
  Notes: Uses HTML parser to extract sections, then delegates to appropriate parsers via SubLanguageManager.
*/

package chunker

import (
	"bytes"
	"log"
	"regexp"
	"strings"

	sitter "github.com/tree-sitter/go-tree-sitter"
	tree_sitter_html "github.com/tree-sitter/tree-sitter-html/bindings/go"
)

// VueParser implements the LanguageParser interface for Vue.js SFC files.
// It extracts <template>, <script>, and <style> sections and parses each appropriately.
type VueParser struct {
	htmlParser     *HTMLParser
	cssParser      *CSSParser
	subLangManager *SubLanguageManager
}

// SetSubLanguageManager implements the SubLanguageAware interface.
func (v *VueParser) SetSubLanguageManager(manager *SubLanguageManager) {
	v.subLangManager = manager
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
func (v *VueParser) ExtractSymbols(tree *sitter.Tree, source []byte) ([]Symbol, error) {
	var symbols []Symbol

	// Initialize sub-parsers if needed
	if v.htmlParser == nil {
		v.htmlParser = &HTMLParser{}
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

// extractSectionsWithPosition extracts sections with original byte/line offsets
func (v *VueParser) extractSectionsWithPosition(source []byte) map[string]sectionInfo {
	sections := make(map[string]sectionInfo)
	templateRe := regexp.MustCompile(`(?s)<template[^>]*>(.*?)</template>`)
	scriptRe := regexp.MustCompile(`(?s)<script([^>]*)>(.*?)</script>`)
	styleRe := regexp.MustCompile(`(?s)<style[^>]*>(.*?)</style>`)

	if match := templateRe.FindSubmatchIndex(source); len(match) >= 4 {
		sections["template"] = sectionInfo{
			name:      "template",
			content:   bytes.TrimSpace(source[match[2]:match[3]]),
			startLine: v.getLineNumber(source, match[0]),
			endLine:   v.getLineNumber(source, match[1]),
			startByte: uint32(match[0]),
			endByte:   uint32(match[1]),
		}
	}

	if match := scriptRe.FindSubmatchIndex(source); len(match) >= 6 {
		attrs := strings.ToLower(string(source[match[2]:match[3]]))
		isTS := strings.Contains(attrs, "lang=\"ts\"") || strings.Contains(attrs, "lang='ts'") ||
			strings.Contains(attrs, "lang=\"tsx\"") || strings.Contains(attrs, "lang='tsx'") ||
			strings.Contains(attrs, "lang=\"typescript\"") || strings.Contains(attrs, "lang='typescript'")

		sections["script"] = sectionInfo{
			name:         "script",
			content:      bytes.TrimSpace(source[match[4]:match[5]]),
			startLine:    v.getLineNumber(source, match[0]),
			endLine:      v.getLineNumber(source, match[1]),
			startByte:    uint32(match[0]),
			endByte:      uint32(match[1]),
			isTypeScript: isTS,
		}
	}

	if match := styleRe.FindSubmatchIndex(source); len(match) >= 4 {
		sections["style"] = sectionInfo{
			name:      "style",
			content:   bytes.TrimSpace(source[match[2]:match[3]]),
			startLine: v.getLineNumber(source, match[0]),
			endLine:   v.getLineNumber(source, match[1]),
			startByte: uint32(match[0]),
			endByte:   uint32(match[1]),
		}
	}

	return sections
}

func (v *VueParser) getLineNumber(source []byte, bytePos int) uint32 {
	line := uint32(1)
	for i := 0; i < bytePos && i < len(source); i++ {
		if source[i] == '\n' {
			line++
		}
	}
	return line
}

func (v *VueParser) parseHTMLSection(section sectionInfo, sectionName string) ([]Symbol, error) {
	htmlTsParser := sitter.NewParser()
	defer htmlTsParser.Close()
	err := htmlTsParser.SetLanguage(v.htmlParser.GetLanguage())
	if err != nil {
		return nil, err
	}
	tree := htmlTsParser.Parse(section.content, nil)
	if tree == nil {
		return nil, nil
	}
	defer tree.Close()

	symbols, err := v.htmlParser.ExtractSymbols(tree, section.content)
	if err != nil {
		return nil, err
	}

	for i := range symbols {
		v.offsetSymbol(&symbols[i], section, sectionName)
	}
	return symbols, nil
}

func (v *VueParser) parseScriptSection(section sectionInfo, sectionName string) ([]Symbol, error) {
	langExt := ".js"
	if section.isTypeScript {
		langExt = ".ts"
	}
	
	if v.subLangManager == nil {
		return nil, nil
	}
	
	parser := v.subLangManager.GetParser(langExt)
	if parser == nil {
		log.Printf("Warning: no parser found for %s script section in Vue", langExt)
		return nil, nil
	}

	jsTsParser := sitter.NewParser()
	defer jsTsParser.Close()
	err := jsTsParser.SetLanguage(parser.GetLanguage())
	if err != nil {
		return nil, err
	}

	tree := jsTsParser.Parse(section.content, nil)
	if tree == nil {
		return nil, nil
	}
	defer tree.Close()

	symbols, err := parser.ExtractSymbols(tree, section.content)
	if err != nil {
		return nil, err
	}

	for i := range symbols {
		v.offsetSymbol(&symbols[i], section, sectionName)
	}
	return symbols, nil
}

func (v *VueParser) parseStyleSection(section sectionInfo, sectionName string) ([]Symbol, error) {
	cssTsParser := sitter.NewParser()
	defer cssTsParser.Close()
	err := cssTsParser.SetLanguage(v.cssParser.GetLanguage())
	if err != nil {
		return nil, err
	}
	tree := cssTsParser.Parse(section.content, nil)
	if tree == nil {
		return nil, nil
	}
	defer tree.Close()

	symbols, err := v.cssParser.ExtractSymbols(tree, section.content)
	if err != nil {
		return nil, err
	}

	for i := range symbols {
		v.offsetSymbol(&symbols[i], section, sectionName)
	}
	return symbols, nil
}

func (v *VueParser) offsetSymbol(sym *Symbol, section sectionInfo, sectionName string) {
	sym.StartLine += section.startLine - 1 // sections content starts at startLine, adjust if internal parser reports relative to 1
	sym.EndLine += section.startLine - 1
	sym.StartByte += section.startByte
	sym.EndByte += section.startByte
	if sym.Parent == "" {
		sym.Parent = sectionName
	}
}

func (v *VueParser) ExtractImports(tree *sitter.Tree, source []byte) ([]string, error) {
	var imports []string
	sections := v.extractSectionsWithPosition(source)

	if scriptSection, ok := sections["script"]; ok {
		langExt := ".js"
		if scriptSection.isTypeScript {
			langExt = ".ts"
		}
		
		if v.subLangManager != nil {
			parser := v.subLangManager.GetParser(langExt)
			if parser != nil {
				jsTsParser := sitter.NewParser()
				defer jsTsParser.Close()
				err := jsTsParser.SetLanguage(parser.GetLanguage())
				if err == nil {
					scriptTree := jsTsParser.Parse(scriptSection.content, nil)
					if scriptTree != nil {
						defer scriptTree.Close()
						scriptImports, _ := parser.ExtractImports(scriptTree, scriptSection.content)
						imports = append(imports, scriptImports...)
					}
				}
			}
		}
	}

	if styleSection, ok := sections["style"]; ok {
		if v.cssParser == nil {
			v.cssParser = &CSSParser{}
		}
		cssTsParser := sitter.NewParser()
		defer cssTsParser.Close()
		err := cssTsParser.SetLanguage(v.cssParser.GetLanguage())
		if err == nil {
			styleTree := cssTsParser.Parse(styleSection.content, nil)
			if styleTree != nil {
				defer styleTree.Close()
				styleImports, _ := v.cssParser.ExtractImports(styleTree, styleSection.content)
				imports = append(imports, styleImports...)
			}
		}
	}

	return imports, nil
}

func (v *VueParser) ExtractMetadata(tree *sitter.Tree, source []byte) map[string]string {
	return make(map[string]string)
}

func (v *VueParser) ExtractUsages(tree *sitter.Tree, source []byte, symbols []Symbol) []ParserSymbolUsage {
	return nil
}
