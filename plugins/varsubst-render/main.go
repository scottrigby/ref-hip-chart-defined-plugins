// Package main implements a render/v1 plugin for Pkl templates.
// This is a reference implementation that demonstrates the render/v1 interface
// for chart-defined plugins in Helm 4.
package main

import (
	"encoding/json"
	"fmt"
	"strings"

	"github.com/extism/go-pdk"
)

// ReleaseInfo contains release metadata passed to render plugins.
type ReleaseInfo struct {
	Name      string `json:"name"`
	Namespace string `json:"namespace"`
	Revision  int    `json:"revision"`
	IsInstall bool   `json:"isInstall"`
	IsUpgrade bool   `json:"isUpgrade"`
	Service   string `json:"service"`
}

// ChartInfo contains chart metadata passed to render plugins.
type ChartInfo struct {
	Name        string `json:"name"`
	Version     string `json:"version"`
	AppVersion  string `json:"appVersion,omitempty"`
	Description string `json:"description,omitempty"`
	Type        string `json:"type,omitempty"`
	IsRoot      bool   `json:"isRoot"`
}

// SubchartInfo contains metadata about a subchart dependency.
type SubchartInfo struct {
	Name    string `json:"name"`
	Version string `json:"version,omitempty"`
	Enabled bool   `json:"enabled"`
}

// CapabilitiesInfo contains Kubernetes cluster capabilities.
type CapabilitiesInfo struct {
	KubeVersion KubeVersionInfo `json:"kubeVersion"`
	APIVersions []string        `json:"apiVersions"`
	HelmVersion string          `json:"helmVersion"`
}

// KubeVersionInfo contains Kubernetes version information.
type KubeVersionInfo struct {
	Version string `json:"version"`
	Major   string `json:"major"`
	Minor   string `json:"minor"`
}

// SourceFile represents a file in the chart.
type SourceFile struct {
	Name string `json:"name"`
	Data []byte `json:"data"`
}

// InputMessageRenderV1 is the input message for render/v1 plugins.
type InputMessageRenderV1 struct {
	Release      ReleaseInfo             `json:"release"`
	Values       map[string]interface{}  `json:"values"`
	Chart        ChartInfo               `json:"chart"`
	Subcharts    map[string]SubchartInfo `json:"subcharts"`
	Files        []SourceFile            `json:"files"`
	Capabilities CapabilitiesInfo        `json:"capabilities"`
	SourceFiles  []SourceFile            `json:"sourceFiles"`
}

// OutputMessageRenderV1 is the output message from render/v1 plugins.
type OutputMessageRenderV1 struct {
	RenderedFiles map[string]string `json:"renderedFiles"`
	Errors        []string          `json:"errors,omitempty"`
}

// HelmPluginMain is the main entry point for the Wasm plugin.
// It processes the input, renders Pkl files, and returns the output.
//
//go:wasmexport helm_plugin_main
func HelmPluginMain() uint32 {
	// Read input from Extism
	inputBytes := pdk.Input()

	// Parse the input message
	var input InputMessageRenderV1
	if err := json.Unmarshal(inputBytes, &input); err != nil {
		return outputError(fmt.Sprintf("failed to parse input: %v", err))
	}

	// Process each source file
	output := OutputMessageRenderV1{
		RenderedFiles: make(map[string]string),
	}

	for _, file := range input.SourceFiles {
		// Only process .pkl files
		if !strings.HasSuffix(file.Name, ".pkl") {
			continue
		}

		// Render the Pkl file
		rendered, err := renderPklFile(file, input)
		if err != nil {
			output.Errors = append(output.Errors, fmt.Sprintf("error rendering %s: %v", file.Name, err))
			continue
		}

		// Generate output filename (replace .pkl with .yaml)
		outputName := strings.TrimSuffix(file.Name, ".pkl") + ".yaml"
		output.RenderedFiles[outputName] = rendered
	}

	// Marshal and return the output
	outputBytes, err := json.Marshal(output)
	if err != nil {
		return outputError(fmt.Sprintf("failed to marshal output: %v", err))
	}

	pdk.Output(outputBytes)
	return 0
}

// renderPklFile renders a single Pkl file using the input context.
// This is a simplified implementation that demonstrates the interface.
// A full implementation would use the Pkl evaluator.
func renderPklFile(file SourceFile, input InputMessageRenderV1) (string, error) {
	// For this reference implementation, we'll do simple template substitution.
	// A real implementation would use the Pkl evaluator to process the file.
	content := string(file.Data)

	// Simple variable substitution for demonstration
	// Replace ${release.name} with actual release name, etc.
	content = strings.ReplaceAll(content, "${release.name}", input.Release.Name)
	content = strings.ReplaceAll(content, "${release.namespace}", input.Release.Namespace)
	content = strings.ReplaceAll(content, "${chart.name}", input.Chart.Name)
	content = strings.ReplaceAll(content, "${chart.version}", input.Chart.Version)

	// Replace values references
	if replicas, ok := input.Values["replicas"]; ok {
		content = strings.ReplaceAll(content, "${values.replicas}", fmt.Sprintf("%v", replicas))
	}
	if image, ok := input.Values["image"].(map[string]interface{}); ok {
		if repo, ok := image["repository"]; ok {
			content = strings.ReplaceAll(content, "${values.image.repository}", fmt.Sprintf("%v", repo))
		}
		if tag, ok := image["tag"]; ok {
			content = strings.ReplaceAll(content, "${values.image.tag}", fmt.Sprintf("%v", tag))
		}
	}

	return content, nil
}

// outputError creates an error output and returns the error code.
func outputError(msg string) uint32 {
	output := OutputMessageRenderV1{
		RenderedFiles: make(map[string]string),
		Errors:        []string{msg},
	}
	outputBytes, _ := json.Marshal(output)
	pdk.Output(outputBytes)
	return 1
}

func main() {}
