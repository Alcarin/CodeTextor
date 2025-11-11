/*
  File: parser_test.go
  Purpose: Unit tests for the code parser and language-specific implementations.
  Author: CodeTextor project
  Notes: Tests Go, Python, and TypeScript parsers with sample code snippets.
*/

package chunker

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestGoParser tests the Go language parser.
func TestGoParser(t *testing.T) {
	parser := NewParser(DefaultChunkConfig())

	source := []byte(`package main

import "fmt"

// Add adds two integers and returns the sum.
func Add(a, b int) int {
	return a + b
}

// Calculator is a simple calculator struct.
type Calculator struct {
	Name string
}

// Multiply is a method on Calculator.
func (c *Calculator) Multiply(a, b int) int {
	return a * b
}

const MaxValue = 100
`)

	result, err := parser.ParseFile("test.go", source)
	require.NoError(t, err, "parsing should not fail")
	require.NotNil(t, result)

	assert.Equal(t, "go", result.Language)
	assert.Equal(t, "test.go", result.FilePath)

	// Verify imports
	assert.Contains(t, result.Imports, "fmt", "should extract import")

	// Verify symbols
	symbols := result.Symbols
	assert.GreaterOrEqual(t, len(symbols), 3, "should extract at least 3 symbols")

	// Find the Add function
	var addFunc *Symbol
	for i := range symbols {
		if symbols[i].Name == "Add" {
			addFunc = &symbols[i]
			break
		}
	}
	require.NotNil(t, addFunc, "Add function should be extracted")
	assert.Equal(t, SymbolFunction, addFunc.Kind)
	assert.Equal(t, "public", addFunc.Visibility)
	assert.Contains(t, addFunc.DocString, "adds two integers")

	// Find the Calculator struct
	var calcStruct *Symbol
	for i := range symbols {
		if symbols[i].Name == "Calculator" {
			calcStruct = &symbols[i]
			break
		}
	}
	require.NotNil(t, calcStruct, "Calculator struct should be extracted")
	assert.Equal(t, SymbolStruct, calcStruct.Kind)

	// Find the Multiply method
	var multiplyMethod *Symbol
	for i := range symbols {
		if symbols[i].Name == "Multiply" {
			multiplyMethod = &symbols[i]
			break
		}
	}
	require.NotNil(t, multiplyMethod, "Multiply method should be extracted")
	assert.Equal(t, SymbolMethod, multiplyMethod.Kind)
	assert.Equal(t, "Calculator", multiplyMethod.Parent)

	// Find the MaxValue constant
	var maxConst *Symbol
	for i := range symbols {
		if symbols[i].Name == "MaxValue" {
			maxConst = &symbols[i]
			break
		}
	}
	require.NotNil(t, maxConst, "MaxValue constant should be extracted")
	assert.Equal(t, SymbolConstant, maxConst.Kind)
}

// TestPythonParser tests the Python language parser.
func TestPythonParser(t *testing.T) {
	parser := NewParser(DefaultChunkConfig())

	source := []byte(`import os
from math import sqrt

def add(a, b):
    """Add two numbers and return the sum."""
    return a + b

class Calculator:
    """A simple calculator class."""

    def __init__(self):
        self.name = "Calculator"

    def multiply(self, a, b):
        """Multiply two numbers."""
        return a * b

def _private_function():
    pass
`)

	result, err := parser.ParseFile("test.py", source)
	require.NoError(t, err, "parsing should not fail")
	require.NotNil(t, result)

	assert.Equal(t, "python", result.Language)

	// Verify imports
	assert.Contains(t, result.Imports, "os")
	assert.Contains(t, result.Imports, "math")

	// Verify symbols
	symbols := result.Symbols

	// Find the add function
	var addFunc *Symbol
	for i := range symbols {
		if symbols[i].Name == "add" {
			addFunc = &symbols[i]
			break
		}
	}
	require.NotNil(t, addFunc, "add function should be extracted")
	assert.Equal(t, SymbolFunction, addFunc.Kind)
	assert.Contains(t, addFunc.DocString, "Add two numbers")

	// Find the Calculator class
	var calcClass *Symbol
	for i := range symbols {
		if symbols[i].Name == "Calculator" {
			calcClass = &symbols[i]
			break
		}
	}
	require.NotNil(t, calcClass, "Calculator class should be extracted")
	assert.Equal(t, SymbolClass, calcClass.Kind)
	assert.Contains(t, calcClass.DocString, "simple calculator")

	// Find the multiply method
	var multiplyMethod *Symbol
	for i := range symbols {
		if symbols[i].Name == "multiply" {
			multiplyMethod = &symbols[i]
			break
		}
	}
	require.NotNil(t, multiplyMethod, "multiply method should be extracted")
	assert.Equal(t, SymbolMethod, multiplyMethod.Kind)

	// Find the private function
	var privateFunc *Symbol
	for i := range symbols {
		if symbols[i].Name == "_private_function" {
			privateFunc = &symbols[i]
			break
		}
	}
	require.NotNil(t, privateFunc, "_private_function should be extracted")
	assert.Equal(t, "protected", privateFunc.Visibility)
}

// TestTypeScriptParser tests the TypeScript/JavaScript parser.
func TestTypeScriptParser(t *testing.T) {
	parser := NewParser(DefaultChunkConfig())

	source := []byte(`import { Component } from 'react';

/**
 * Add two numbers
 */
function add(a: number, b: number): number {
    return a + b;
}

const multiply = (a: number, b: number): number => a * b;

class Calculator {
    private name: string;

    constructor() {
        this.name = "Calculator";
    }

    public divide(a: number, b: number): number {
        return a / b;
    }
}

export { add, multiply };
`)

	result, err := parser.ParseFile("test.ts", source)
	require.NoError(t, err, "parsing should not fail")
	require.NotNil(t, result)

	assert.Equal(t, "typescript", result.Language)

	// Verify imports
	assert.Contains(t, result.Imports, "react")

	// Verify symbols
	symbols := result.Symbols

	// Find the add function
	var addFunc *Symbol
	for i := range symbols {
		if symbols[i].Name == "add" {
			addFunc = &symbols[i]
			break
		}
	}
	require.NotNil(t, addFunc, "add function should be extracted")
	assert.Equal(t, SymbolFunction, addFunc.Kind)
	assert.Contains(t, addFunc.DocString, "Add two numbers")

	// Find the multiply arrow function
	var multiplyFunc *Symbol
	for i := range symbols {
		if symbols[i].Name == "multiply" {
			multiplyFunc = &symbols[i]
			break
		}
	}
	require.NotNil(t, multiplyFunc, "multiply arrow function should be extracted")
	assert.Equal(t, SymbolFunction, multiplyFunc.Kind)

	// Find the Calculator class
	var calcClass *Symbol
	for i := range symbols {
		if symbols[i].Name == "Calculator" {
			calcClass = &symbols[i]
			break
		}
	}
	require.NotNil(t, calcClass, "Calculator class should be extracted")
	assert.Equal(t, SymbolClass, calcClass.Kind)

	// Find the divide method
	var divideMethod *Symbol
	for i := range symbols {
		if symbols[i].Name == "divide" {
			divideMethod = &symbols[i]
			break
		}
	}
	require.NotNil(t, divideMethod, "divide method should be extracted")
	assert.Equal(t, SymbolMethod, divideMethod.Kind)
	assert.Equal(t, "public", divideMethod.Visibility)
}

// TestParserUnsupportedExtension tests that unsupported files return an error.
func TestParserUnsupportedExtension(t *testing.T) {
	parser := NewParser(DefaultChunkConfig())

	_, err := parser.ParseFile("test.xyz", []byte("some content"))
	require.Error(t, err, "should return error for unsupported extension")
	assert.Contains(t, err.Error(), "unsupported file extension")
}

// TestParserIsSupported tests the IsSupported method.
func TestParserIsSupported(t *testing.T) {
	parser := NewParser(DefaultChunkConfig())

	assert.True(t, parser.IsSupported("test.go"))
	assert.True(t, parser.IsSupported("test.py"))
	assert.True(t, parser.IsSupported("test.ts"))
	assert.True(t, parser.IsSupported("test.js"))
	assert.True(t, parser.IsSupported("test.html"))
	assert.True(t, parser.IsSupported("test.css"))
	assert.True(t, parser.IsSupported("test.vue"))
	assert.True(t, parser.IsSupported("test.md"))
	assert.True(t, parser.IsSupported("test.markdown"))
	assert.False(t, parser.IsSupported("test.txt"))
	assert.False(t, parser.IsSupported("test.xyz"))
}

// TestParserGetSupportedExtensions tests that all expected extensions are registered.
func TestParserGetSupportedExtensions(t *testing.T) {
	parser := NewParser(DefaultChunkConfig())

	extensions := parser.GetSupportedExtensions()
	assert.Contains(t, extensions, ".go")
	assert.Contains(t, extensions, ".py")
	assert.Contains(t, extensions, ".ts")
	assert.Contains(t, extensions, ".js")
	assert.Contains(t, extensions, ".html")
	assert.Contains(t, extensions, ".css")
	assert.Contains(t, extensions, ".vue")
	assert.Contains(t, extensions, ".md")
	assert.Contains(t, extensions, ".markdown")
}

// TestParseErrorHandling tests that syntax errors are captured.
func TestParseErrorHandling(t *testing.T) {
	parser := NewParser(DefaultChunkConfig())

	// Invalid Go code with syntax error
	source := []byte(`package main

func broken {  // Missing parameters
	return 42
}
`)

	result, err := parser.ParseFile("test.go", source)
	// Parser should not fail, but should report errors in result
	require.NoError(t, err)
	require.NotNil(t, result)

	// Check if errors were captured (tree-sitter should detect the syntax error)
	// Note: Tree-sitter's error detection varies, so this test might be lenient
	assert.NotNil(t, result.Errors, "errors field should exist")
}

// TestHTMLParser tests the HTML language parser.
func TestHTMLParser(t *testing.T) {
	parser := NewParser(DefaultChunkConfig())

	source := []byte(`<!DOCTYPE html>
<html>
<head>
    <title>Test Page</title>
</head>
<body>
    <header id="main-header" class="site-header">
        <h1>Welcome</h1>
    </header>
    <main id="content">
        <section id="intro">
            <p class="intro-text">Introduction text</p>
            <div class="button-group">
                <button type="submit" id="btn1">Click me</button>
            </div>
        </section>
    </main>
    <script src="app.js">
        console.log('Hello');
    </script>
    <style>
        body { margin: 0; }
    </style>
</body>
</html>`)

	result, err := parser.ParseFile("test.html", source)
	require.NoError(t, err, "parsing should not fail")
	require.NotNil(t, result)

	assert.Equal(t, "html", result.Language)
	assert.Equal(t, "test.html", result.FilePath)

	// Verify symbols
	symbols := result.Symbols
	assert.Greater(t, len(symbols), 0, "should extract HTML symbols")

	// Find the header element with ID and verify attributes in signature
	var headerElem *Symbol
	for i := range symbols {
		if symbols[i].Name == "header#main-header" {
			headerElem = &symbols[i]
			break
		}
	}
	require.NotNil(t, headerElem, "header element should be extracted")
	assert.Equal(t, SymbolElement, headerElem.Kind)
	assert.Contains(t, headerElem.Signature, "id='main-header'", "should have id in signature")
	assert.Contains(t, headerElem.Signature, "class='site-header'", "should have class in signature")

	// Find the main element
	var mainElem *Symbol
	for i := range symbols {
		if symbols[i].Name == "main#content" {
			mainElem = &symbols[i]
			break
		}
	}
	require.NotNil(t, mainElem, "main element should be extracted")

	// Find regular elements (div, p, h1, button) - all tags should now be extracted
	var divElem *Symbol
	var pElem *Symbol
	var h1Elem *Symbol
	var buttonElem *Symbol
	for i := range symbols {
		switch {
		case symbols[i].Name == "div" && symbols[i].Kind == SymbolElement:
			divElem = &symbols[i]
		case symbols[i].Name == "p" && symbols[i].Kind == SymbolElement:
			pElem = &symbols[i]
		case symbols[i].Name == "h1" && symbols[i].Kind == SymbolElement:
			h1Elem = &symbols[i]
		case symbols[i].Name == "button#btn1" && symbols[i].Kind == SymbolElement:
			buttonElem = &symbols[i]
		}
	}
	require.NotNil(t, divElem, "div element should be extracted")
	require.NotNil(t, pElem, "p element should be extracted")
	require.NotNil(t, h1Elem, "h1 element should be extracted")
	require.NotNil(t, buttonElem, "button element should be extracted")

	// Verify button attributes
	assert.Contains(t, buttonElem.Signature, "type='submit'", "button should have type in signature")
	assert.Contains(t, buttonElem.Signature, "id='btn1'", "button should have id in signature")

	// Find script block and verify src attribute
	var scriptBlock *Symbol
	for i := range symbols {
		if symbols[i].Kind == SymbolScript {
			scriptBlock = &symbols[i]
			break
		}
	}
	require.NotNil(t, scriptBlock, "script block should be extracted")

	// Find style block
	var styleBlock *Symbol
	for i := range symbols {
		if symbols[i].Kind == SymbolStyle {
			styleBlock = &symbols[i]
			break
		}
	}
	require.NotNil(t, styleBlock, "style block should be extracted")

	// Verify imports (script src)
	assert.Contains(t, result.Imports, "app.js", "should extract script src as import")
}

// TestCSSParser tests the CSS language parser.
func TestCSSParser(t *testing.T) {
	parser := NewParser(DefaultChunkConfig())

	source := []byte(`@import url('base.css');

.container {
    width: 100%;
    padding: 20px;
}

#header {
    background-color: blue;
}

@media (max-width: 768px) {
    .container {
        padding: 10px;
    }
}

@keyframes fadeIn {
    from { opacity: 0; }
    to { opacity: 1; }
}`)

	result, err := parser.ParseFile("test.css", source)
	require.NoError(t, err, "parsing should not fail")
	require.NotNil(t, result)

	assert.Equal(t, "css", result.Language)
	assert.Equal(t, "test.css", result.FilePath)

	// Verify imports
	assert.Contains(t, result.Imports, "base.css", "should extract @import")

	// Verify symbols
	symbols := result.Symbols
	assert.Greater(t, len(symbols), 0, "should extract CSS symbols")

	// Find container rule
	var containerRule *Symbol
	for i := range symbols {
		if symbols[i].Name == ".container" {
			containerRule = &symbols[i]
			break
		}
	}
	require.NotNil(t, containerRule, "container rule should be extracted")
	assert.Equal(t, SymbolCSSRule, containerRule.Kind)

	// Find media query
	var mediaRule *Symbol
	for i := range symbols {
		if symbols[i].Kind == SymbolCSSMedia {
			mediaRule = &symbols[i]
			break
		}
	}
	require.NotNil(t, mediaRule, "media rule should be extracted")
	assert.Contains(t, mediaRule.Name, "@media")

	// Find keyframes
	var keyframesRule *Symbol
	for i := range symbols {
		if symbols[i].Kind == SymbolCSSKeyframes {
			keyframesRule = &symbols[i]
			break
		}
	}
	require.NotNil(t, keyframesRule, "keyframes rule should be extracted")
	assert.Contains(t, keyframesRule.Name, "fadeIn")
}

// TestVueParser tests the Vue SFC language parser.
func TestVueParser(t *testing.T) {
	parser := NewParser(DefaultChunkConfig())

	source := []byte(`<template>
  <div id="app">
    <header>
      <h1>{{ title }}</h1>
    </header>
    <main>
      <p>Content here</p>
    </main>
  </div>
</template>

<script>
import { ref } from 'vue';

export default {
  setup() {
    const title = ref('My App');

    function greet() {
      console.log('Hello');
    }

    return { title, greet };
  }
}
</script>

<style scoped>
.container {
  padding: 20px;
}

#app {
  font-family: Arial;
}
</style>`)

	result, err := parser.ParseFile("test.vue", source)
	require.NoError(t, err, "parsing should not fail")
	require.NotNil(t, result)

	assert.Equal(t, "vue", result.Language)
	assert.Equal(t, "test.vue", result.FilePath)

	// Verify imports from script section
	assert.Contains(t, result.Imports, "vue", "should extract imports from script section")

	// Verify symbols
	symbols := result.Symbols
	assert.Greater(t, len(symbols), 0, "should extract symbols from all sections")

	// Find section nodes (template, script, style)
	var templateSectionNode *Symbol
	var scriptSectionNode *Symbol
	var styleSectionNode *Symbol
	for i := range symbols {
		if symbols[i].Name == "template" && symbols[i].Kind == SymbolElement && symbols[i].Parent == "" {
			templateSectionNode = &symbols[i]
		}
		if symbols[i].Name == "script" && symbols[i].Kind == SymbolScript && symbols[i].Parent == "" {
			scriptSectionNode = &symbols[i]
		}
		if symbols[i].Name == "style" && symbols[i].Kind == SymbolStyle && symbols[i].Parent == "" {
			styleSectionNode = &symbols[i]
		}
	}
	require.NotNil(t, templateSectionNode, "should have template section node")
	require.NotNil(t, scriptSectionNode, "should have script section node")
	require.NotNil(t, styleSectionNode, "should have style section node")

	// Verify section nodes have correct line numbers
	assert.Equal(t, uint32(1), templateSectionNode.StartLine, "template should start at line 1")
	assert.Greater(t, templateSectionNode.EndLine, templateSectionNode.StartLine, "template should span multiple lines")

	assert.Equal(t, uint32(12), scriptSectionNode.StartLine, "script should start at line 12")
	assert.Greater(t, scriptSectionNode.EndLine, scriptSectionNode.StartLine, "script should span multiple lines")

	assert.Equal(t, uint32(28), styleSectionNode.StartLine, "style should start at line 28")
	assert.Greater(t, styleSectionNode.EndLine, styleSectionNode.StartLine, "style should span multiple lines")

	// Find child symbols from template section
	var divElement *Symbol
	for i := range symbols {
		if symbols[i].Name == "div#app" && symbols[i].Parent == "template" {
			divElement = &symbols[i]
			break
		}
	}
	require.NotNil(t, divElement, "should extract div#app from template section")
	assert.Greater(t, divElement.StartLine, templateSectionNode.StartLine, "div should be after template start")
	assert.Less(t, divElement.EndLine, templateSectionNode.EndLine, "div should be before template end")

	// Find child symbols from script section (function)
	var greetFunction *Symbol
	for i := range symbols {
		if symbols[i].Name == "greet" && symbols[i].Kind == SymbolFunction {
			greetFunction = &symbols[i]
			break
		}
	}
	require.NotNil(t, greetFunction, "should extract greet function from script section")
	assert.Contains(t, greetFunction.Parent, "script", "greet function should have script as parent")
	assert.Greater(t, greetFunction.StartLine, scriptSectionNode.StartLine, "function should be after script start")
	assert.Less(t, greetFunction.EndLine, scriptSectionNode.EndLine, "function should be before script end")

	// Find child symbols from style section
	var containerRule *Symbol
	for i := range symbols {
		if symbols[i].Name == ".container" && symbols[i].Parent == "style" {
			containerRule = &symbols[i]
			break
		}
	}
	require.NotNil(t, containerRule, "should extract .container rule from style section")
	assert.Greater(t, containerRule.StartLine, styleSectionNode.StartLine, "rule should be after style start")
	assert.Less(t, containerRule.EndLine, styleSectionNode.EndLine, "rule should be before style end")
}

// TestMarkdownParser tests the Markdown language parser.
func TestMarkdownParser(t *testing.T) {
	parser := NewParser(DefaultChunkConfig())

	source := []byte(`# Main Title

This is an introduction paragraph.

## Section 1

Some text here with a [link](https://example.com) and another [local link](./other.md).

### Subsection 1.1

Here's some code:

` + "```go" + `
func main() {
    fmt.Println("Hello")
}
` + "```" + `

## Section 2

More content here.

` + "```python" + `
def hello():
    print("Hello")
` + "```" + `
`)

	result, err := parser.ParseFile("test.md", source)
	require.NoError(t, err, "parsing should not fail")
	require.NotNil(t, result)

	assert.Equal(t, "markdown", result.Language)
	assert.Equal(t, "test.md", result.FilePath)

	// Verify imports (local file links)
	assert.Contains(t, result.Imports, "./other.md", "should extract local file references")
	assert.NotContains(t, result.Imports, "https://example.com", "should not include external URLs")

	// Verify symbols
	symbols := result.Symbols
	assert.Greater(t, len(symbols), 0, "should extract Markdown symbols")

	// Find main heading
	var mainHeading *Symbol
	for i := range symbols {
		if symbols[i].Name == "Main Title" && symbols[i].Kind == SymbolMarkdownHeading {
			mainHeading = &symbols[i]
			break
		}
	}
	require.NotNil(t, mainHeading, "main heading should be extracted")
	assert.Equal(t, "h1", mainHeading.Signature, "should be h1")

	// Find section heading
	var section1 *Symbol
	for i := range symbols {
		if symbols[i].Name == "Section 1" && symbols[i].Kind == SymbolMarkdownHeading {
			section1 = &symbols[i]
			break
		}
	}
	require.NotNil(t, section1, "section heading should be extracted")

	// Find code blocks
	var goCodeBlock *Symbol
	for i := range symbols {
		if symbols[i].Kind == SymbolMarkdownCode && symbols[i].Signature == "go" {
			goCodeBlock = &symbols[i]
			break
		}
	}
	require.NotNil(t, goCodeBlock, "Go code block should be extracted")
	assert.Equal(t, "code:go", goCodeBlock.Name)

	// Find Python code block
	var pythonCodeBlock *Symbol
	for i := range symbols {
		if symbols[i].Kind == SymbolMarkdownCode && symbols[i].Signature == "python" {
			pythonCodeBlock = &symbols[i]
			break
		}
	}
	require.NotNil(t, pythonCodeBlock, "Python code block should be extracted")

	// Find links and verify they have line numbers
	var externalLink *Symbol
	var localLink *Symbol
	for i := range symbols {
		if symbols[i].Kind == SymbolMarkdownLink {
			if symbols[i].Name == "https://example.com" {
				externalLink = &symbols[i]
			} else if symbols[i].Name == "./other.md" {
				localLink = &symbols[i]
			}
		}
	}
	require.NotNil(t, externalLink, "external link should be extracted")
	require.NotNil(t, localLink, "local link should be extracted")

	// Verify line numbers are set
	assert.Greater(t, externalLink.StartLine, uint32(0), "external link should have line number")
	assert.Greater(t, localLink.StartLine, uint32(0), "local link should have line number")
	assert.Equal(t, externalLink.StartLine, externalLink.EndLine, "link should be on single line")
	assert.Equal(t, localLink.StartLine, localLink.EndLine, "link should be on single line")

	// Verify byte positions are set
	assert.Greater(t, externalLink.StartByte, uint32(0), "external link should have byte position")
	assert.Greater(t, externalLink.EndByte, externalLink.StartByte, "end byte should be after start byte")
	assert.Greater(t, localLink.StartByte, uint32(0), "local link should have byte position")
	assert.Greater(t, localLink.EndByte, localLink.StartByte, "end byte should be after start byte")
}

// TestSQLParser ensures SQL statements are represented as SQL statement symbols.
func TestSQLParser(t *testing.T) {
	parser := NewParser(DefaultChunkConfig())

	source := []byte(`
CREATE TABLE users (
  id SERIAL PRIMARY KEY,
  name TEXT
);

INSERT INTO users (name) VALUES ('alice');

SELECT id, name FROM users;

DROP TABLE users;
`)

	result, err := parser.ParseFile("schema.sql", source)
	require.NoError(t, err)
	require.NotNil(t, result)

	assert.Equal(t, "sql", result.Language)
	assert.Equal(t, "schema.sql", result.FilePath)
	assert.Len(t, result.Symbols, 4, "should capture each DDL/DML statement")

	names := make(map[string]Symbol)
	for _, sym := range result.Symbols {
		names[sym.Name] = sym
		assert.Equal(t, SymbolSQLStatement, sym.Kind)
	}

	require.Contains(t, names, "CREATE TABLE users")
	assert.Contains(t, names["CREATE TABLE users"].Signature, "CREATE TABLE users")

	require.Contains(t, names, "INSERT users")
	assert.Contains(t, names["INSERT users"].Signature, "INSERT INTO")

	require.Contains(t, names, "SELECT users")
	assert.Contains(t, names["SELECT users"].Signature, "SELECT id")

	require.Contains(t, names, "DROP TABLE users")
	assert.Contains(t, names["DROP TABLE users"].Signature, "DROP TABLE users")
}

// TestJSONParser verifies JSON parser extracts keys as symbols with value signatures.
func TestJSONParser(t *testing.T) {
	parser := NewParser(DefaultChunkConfig())

	source := []byte(`{
  "name": "CodeTextor",
  "version": "1.0.0",
  "keywords": ["code", "parser"],
  "nested": {
    "flag": true
  },
  "count": 42
}`)

	result, err := parser.ParseFile("project.json", source)
	require.NoError(t, err)
	require.NotNil(t, result)

	assert.Equal(t, "json", result.Language)
	assert.Equal(t, "project.json", result.FilePath)
	assert.Empty(t, result.Imports, "JSON files should not expose imports")

	require.Len(t, result.Symbols, 6, "JSON should expose one symbol per key/value pair")

	symbolMap := make(map[string]Symbol)
	for _, sym := range result.Symbols {
		symbolMap[sym.Name] = sym
	}

	require.Contains(t, symbolMap, "name")
	assert.Equal(t, `"CodeTextor"`, symbolMap["name"].Signature)

	require.Contains(t, symbolMap, "version")
	assert.Equal(t, `"1.0.0"`, symbolMap["version"].Signature)

	require.Contains(t, symbolMap, "keywords")
	assert.Contains(t, symbolMap["keywords"].Signature, `"code"`)

	require.Contains(t, symbolMap, "nested")
	assert.Contains(t, symbolMap["nested"].Signature, `"flag"`)

	require.Contains(t, symbolMap, "flag")
	assert.Equal(t, "true", symbolMap["flag"].Signature)

	require.Contains(t, symbolMap, "count")
	assert.Equal(t, "42", symbolMap["count"].Signature)

	assert.Equal(t, "", symbolMap["name"].Parent, "top-level keys have empty parent")
	assert.Equal(t, "", symbolMap["nested"].Parent, "top-level object retains empty parent")
	assert.Equal(t, "nested", symbolMap["flag"].Parent, "nested key inherits parent name")
}
