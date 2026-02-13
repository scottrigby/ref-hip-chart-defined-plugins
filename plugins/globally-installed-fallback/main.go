// Package main implements a test render/v1 plugin for verifying
// the fallback to non-versioned (globally installed) plugin paths.
//
// This plugin processes .fallback files and renders a ConfigMap
// that proves it was loaded from the globally installed path.
package main

import (
	"encoding/json"
	"fmt"

	"github.com/extism/go-pdk"
)

// SourceFile represents a file in the chart.
type SourceFile struct {
	Name string `json:"name"`
	Data []byte `json:"data"`
}

// InputMessageRenderV1 is the input message for render/v1 plugins.
type InputMessageRenderV1 struct {
	Release      map[string]interface{} `json:"release"`
	Values       map[string]interface{} `json:"values"`
	Chart        map[string]interface{} `json:"chart"`
	Subcharts    map[string]interface{} `json:"subcharts"`
	Files        []SourceFile           `json:"files"`
	Capabilities map[string]interface{} `json:"capabilities"`
	SourceFiles  []SourceFile           `json:"sourceFiles"`
}

// OutputMessageRenderV1 is the output message from render/v1 plugins.
type OutputMessageRenderV1 struct {
	RenderedFiles       map[string]string `json:"renderedFiles"`
	ModifiedSourceFiles []SourceFile      `json:"modifiedSourceFiles,omitempty"`
	Errors              []string          `json:"errors,omitempty"`
}

//go:wasmexport helm_plugin_main
func HelmPluginMain() uint32 {
	pdk.Log(pdk.LogDebug, "globally-installed-fallback plugin starting")

	// Read input from Extism
	inputBytes := pdk.Input()

	// Parse the input message
	var input InputMessageRenderV1
	if err := json.Unmarshal(inputBytes, &input); err != nil {
		return outputError(fmt.Sprintf("failed to parse input: %v", err))
	}

	output := OutputMessageRenderV1{
		RenderedFiles: make(map[string]string),
	}

	// Process each source file
	for _, file := range input.SourceFiles {
		pdk.Log(pdk.LogDebug, fmt.Sprintf("Processing file: %s", file.Name))

		// Render a ConfigMap that proves fallback worked
		content := fmt.Sprintf(`# Rendered by globally-installed-fallback plugin
# This proves the fallback to non-versioned path worked
apiVersion: v1
kind: ConfigMap
metadata:
  name: fallback-test-result
data:
  plugin: "globally-installed-fallback"
  version: "0.1.0"
  fallbackWorked: "true"
  sourceFile: "%s"
  originalContent: |
    %s
`, file.Name, string(file.Data))

		output.RenderedFiles["fallback-test-result.yaml"] = content
	}

	// Marshal and return the output
	outputBytes, err := json.Marshal(output)
	if err != nil {
		return outputError(fmt.Sprintf("failed to marshal output: %v", err))
	}

	pdk.Output(outputBytes)
	pdk.Log(pdk.LogDebug, "globally-installed-fallback plugin completed")
	return 0
}

func outputError(msg string) uint32 {
	pdk.Log(pdk.LogError, msg)
	output := OutputMessageRenderV1{
		RenderedFiles: make(map[string]string),
		Errors:        []string{msg},
	}
	outputBytes, _ := json.Marshal(output)
	pdk.Output(outputBytes)
	return 1
}

func main() {}
