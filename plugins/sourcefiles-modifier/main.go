// Package main implements a test render/v1 plugin that modifies SourceFiles
// to verify sequential plugin handoff works correctly.
//
// This plugin demonstrates:
// - Removing files from SourceFiles (first file is removed)
// - Modifying file content (second file gets "[MODIFIED]" prefix)
// - Renaming files (third file extension changed from .test to .renamed)
// - Adding new files (adds file4.test for next plugin)
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
	pdk.Log(pdk.LogDebug, "sourcefiles-modifier plugin starting")

	// Read input from Extism
	inputBytes := pdk.Input()

	// Parse the input message
	var input InputMessageRenderV1
	if err := json.Unmarshal(inputBytes, &input); err != nil {
		return outputError(fmt.Sprintf("failed to parse input: %v", err))
	}

	pdk.Log(pdk.LogDebug, fmt.Sprintf("Received %d source files", len(input.SourceFiles)))

	// Process the source files and create modified set
	output := OutputMessageRenderV1{
		RenderedFiles:       make(map[string]string),
		ModifiedSourceFiles: make([]SourceFile, 0),
	}

	// Track what we've done for the rendered output
	var actions []string

	for i, file := range input.SourceFiles {
		pdk.Log(pdk.LogDebug, fmt.Sprintf("Processing file %d: %s", i, file.Name))

		switch {
		case i == 0:
			// Remove the first file by not adding it to ModifiedSourceFiles
			actions = append(actions, fmt.Sprintf("REMOVED: %s", file.Name))
			pdk.Log(pdk.LogDebug, fmt.Sprintf("Removing file: %s", file.Name))

		case i == 1:
			// Modify the content of the second file
			newContent := "[MODIFIED BY PLUGIN 1]\n" + string(file.Data)
			output.ModifiedSourceFiles = append(output.ModifiedSourceFiles, SourceFile{
				Name: file.Name,
				Data: []byte(newContent),
			})
			actions = append(actions, fmt.Sprintf("MODIFIED: %s", file.Name))
			pdk.Log(pdk.LogDebug, fmt.Sprintf("Modified file: %s", file.Name))

		case i == 2:
			// Change the extension of the third file
			newName := strings.TrimSuffix(file.Name, ".test") + ".renamed"
			output.ModifiedSourceFiles = append(output.ModifiedSourceFiles, SourceFile{
				Name: newName,
				Data: file.Data,
			})
			actions = append(actions, fmt.Sprintf("RENAMED: %s -> %s", file.Name, newName))
			pdk.Log(pdk.LogDebug, fmt.Sprintf("Renamed file: %s -> %s", file.Name, newName))

		default:
			// Pass through any other files unchanged
			output.ModifiedSourceFiles = append(output.ModifiedSourceFiles, file)
			actions = append(actions, fmt.Sprintf("PASSED: %s", file.Name))
		}
	}

	// Add a new file for the next plugin to process
	newFileName := "templates/file4.test"
	newFileContent := "# This file was added by sourcefiles-modifier plugin\nkey: added-by-plugin-1"
	output.ModifiedSourceFiles = append(output.ModifiedSourceFiles, SourceFile{
		Name: newFileName,
		Data: []byte(newFileContent),
	})
	actions = append(actions, fmt.Sprintf("ADDED: %s", newFileName))
	pdk.Log(pdk.LogDebug, fmt.Sprintf("Added new file: %s", newFileName))

	// Create a summary manifest that documents what we did
	summaryContent := fmt.Sprintf(`# SourceFiles Modifier Plugin Summary
# This manifest documents the modifications made to source files
apiVersion: v1
kind: ConfigMap
metadata:
  name: sourcefiles-modifier-summary
data:
  actions: |
%s
  filesReceived: "%d"
  filesOutput: "%d"
`, formatActions(actions), len(input.SourceFiles), len(output.ModifiedSourceFiles))

	output.RenderedFiles["sourcefiles-modifier-summary.yaml"] = summaryContent

	// Marshal and return the output
	outputBytes, err := json.Marshal(output)
	if err != nil {
		return outputError(fmt.Sprintf("failed to marshal output: %v", err))
	}

	pdk.Output(outputBytes)
	pdk.Log(pdk.LogDebug, "sourcefiles-modifier plugin completed successfully")
	return 0
}

func formatActions(actions []string) string {
	var sb strings.Builder
	for _, action := range actions {
		sb.WriteString("    - ")
		sb.WriteString(action)
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
