// Package main implements a test render/v1 plugin that processes .test files
// and reports what files it received. This is used to verify that the
// sourcefiles-modifier plugin correctly modified the SourceFiles.
package main

import (
	"encoding/json"
	"fmt"
	"strings"

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
	pdk.Log(pdk.LogDebug, "test-processor plugin starting")

	// Read input from Extism
	inputBytes := pdk.Input()

	// Parse the input message
	var input InputMessageRenderV1
	if err := json.Unmarshal(inputBytes, &input); err != nil {
		return outputError(fmt.Sprintf("failed to parse input: %v", err))
	}

	pdk.Log(pdk.LogDebug, fmt.Sprintf("test-processor received %d source files", len(input.SourceFiles)))

	output := OutputMessageRenderV1{
		RenderedFiles: make(map[string]string),
	}

	// Process each source file and render it
	var fileList []string
	for _, file := range input.SourceFiles {
		pdk.Log(pdk.LogDebug, fmt.Sprintf("Processing file: %s", file.Name))
		fileList = append(fileList, file.Name)

		// Render each file as a ConfigMap showing what we received
		outputName := strings.TrimSuffix(file.Name, ".test") + ".yaml"
		content := fmt.Sprintf(`# Rendered by test-processor plugin
# Original file: %s
apiVersion: v1
kind: ConfigMap
metadata:
  name: %s
data:
  originalContent: |
%s
`, file.Name, sanitizeName(file.Name), indentContent(string(file.Data)))
		output.RenderedFiles[outputName] = content
	}

	// Create a summary ConfigMap
	summaryContent := fmt.Sprintf(`# Test Processor Plugin Summary
# Documents what files were received from the previous plugin
apiVersion: v1
kind: ConfigMap
metadata:
  name: test-processor-summary
data:
  filesReceived: "%d"
  fileList: |
%s
`, len(input.SourceFiles), formatFileList(fileList))

	output.RenderedFiles["test-processor-summary.yaml"] = summaryContent

	// Marshal and return the output
	outputBytes, err := json.Marshal(output)
	if err != nil {
		return outputError(fmt.Sprintf("failed to marshal output: %v", err))
	}

	pdk.Output(outputBytes)
	pdk.Log(pdk.LogDebug, "test-processor plugin completed successfully")
	return 0
}

func sanitizeName(name string) string {
	// Convert path to a valid k8s name
	name = strings.ReplaceAll(name, "/", "-")
	name = strings.ReplaceAll(name, ".", "-")
	name = strings.TrimPrefix(name, "templates-")
	return name
}

func indentContent(content string) string {
	lines := strings.Split(content, "\n")
	var sb strings.Builder
	for _, line := range lines {
		sb.WriteString("    ")
		sb.WriteString(line)
		sb.WriteString("\n")
	}
	return sb.String()
}

func formatFileList(files []string) string {
	var sb strings.Builder
	for _, f := range files {
		sb.WriteString("    - ")
		sb.WriteString(f)
		sb.WriteString("\n")
	}
	return sb.String()
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
