package main

import (
	"encoding/json"

	pdk "github.com/extism/go-pdk"
)

// InputMessage represents the render/v1 input
type InputMessage struct {
	Release     map[string]interface{}   `json:"release"`
	Values      map[string]interface{}   `json:"values"`
	Chart       map[string]interface{}   `json:"chart"`
	SourceFiles []SourceFile             `json:"sourceFiles"`
}

// SourceFile represents a file to render
type SourceFile struct {
	Name string `json:"name"`
	Data []byte `json:"data"`
}

// OutputMessage represents the render/v1 output
type OutputMessage struct {
	RenderedFiles map[string]string `json:"renderedFiles"`
	Errors        []string          `json:"errors,omitempty"`
}

//go:wasmexport helm_plugin_main
func HelmPluginMain() uint32 {
	// Read input
	inputData := pdk.Input()
	if len(inputData) == 0 {
		setError("no input provided")
		return 1
	}

	var input InputMessage
	if err := json.Unmarshal(inputData, &input); err != nil {
		setError("failed to parse input: " + err.Error())
		return 1
	}

	// Process each source file - just echo the content with a header
	output := OutputMessage{
		RenderedFiles: make(map[string]string),
	}

	releaseName := "unknown"
	if name, ok := input.Release["name"].(string); ok {
		releaseName = name
	}

	for _, sf := range input.SourceFiles {
		// Change extension from .echo to .yaml
		outName := sf.Name[:len(sf.Name)-5] + ".yaml"
		content := "# Rendered by echo-render plugin\n"
		content += "# Release: " + releaseName + "\n"
		content += string(sf.Data)
		output.RenderedFiles[outName] = content
	}

	// Write output
	outputData, err := json.Marshal(output)
	if err != nil {
		setError("failed to marshal output: " + err.Error())
		return 1
	}

	pdk.Output(outputData)
	return 0
}

func setError(msg string) {
	output := OutputMessage{
		RenderedFiles: make(map[string]string),
		Errors:        []string{msg},
	}
	outputData, _ := json.Marshal(output)
	pdk.Output(outputData)
}

func main() {}
