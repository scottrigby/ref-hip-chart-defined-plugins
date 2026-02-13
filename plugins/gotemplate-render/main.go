// Package main implements a render/v1 plugin for Go templates.
// This is a reference implementation that demonstrates using gotemplate
// as a render/v1 plugin for chart-defined plugins in Helm 4.
//
// Note: This is a simplified implementation. A full implementation would
// need to include all Sprig functions and Helm-specific template functions.
package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"path"
	"strings"
	"text/template"

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

// CapabilitiesInfo contains Kubernetes cluster capabilities.
type CapabilitiesInfo struct {
	KubeVersion map[string]interface{} `json:"kubeVersion"`
	APIVersions []string               `json:"apiVersions"`
	HelmVersion string                 `json:"helmVersion"`
}

// SourceFile represents a file in the chart.
type SourceFile struct {
	Name string `json:"name"`
	Data []byte `json:"data"`
}

// InputMessageRenderV1 is the input message for render/v1 plugins.
type InputMessageRenderV1 struct {
	Release      ReleaseInfo            `json:"release"`
	Values       map[string]interface{} `json:"values"`
	Chart        ChartInfo              `json:"chart"`
	Subcharts    map[string]interface{} `json:"subcharts"`
	Files        []SourceFile           `json:"files"`
	Capabilities CapabilitiesInfo       `json:"capabilities"`
	SourceFiles  []SourceFile           `json:"sourceFiles"`
}

// OutputMessageRenderV1 is the output message from render/v1 plugins.
type OutputMessageRenderV1 struct {
	RenderedFiles       map[string]string `json:"renderedFiles"`
	ModifiedSourceFiles []SourceFile      `json:"modifiedSourceFiles,omitempty"`
	Errors              []string          `json:"errors,omitempty"`
}

// TemplateData holds all data available to templates.
type TemplateData struct {
	Release      ReleaseInfo
	Values       map[string]interface{}
	Chart        ChartInfo
	Subcharts    map[string]interface{}
	Files        *Files
	Capabilities CapabilitiesInfo
	Template     TemplateInfo
}

// TemplateInfo contains info about the current template.
type TemplateInfo struct {
	Name     string
	BasePath string
}

// Files provides access to non-template files.
type Files struct {
	files map[string][]byte
}

// Get returns the content of a file.
func (f *Files) Get(name string) string {
	if data, ok := f.files[name]; ok {
		return string(data)
	}
	return ""
}

// GetBytes returns the content of a file as bytes.
func (f *Files) GetBytes(name string) []byte {
	return f.files[name]
}

// Glob returns files matching a pattern.
func (f *Files) Glob(pattern string) map[string][]byte {
	result := make(map[string][]byte)
	for name, data := range f.files {
		matched, err := path.Match(pattern, name)
		if err == nil && matched {
			result[name] = data
		}
	}
	return result
}

// AsConfig returns files as YAML-formatted config data.
func (f *Files) AsConfig() map[string]string {
	result := make(map[string]string)
	for name, data := range f.files {
		result[path.Base(name)] = string(data)
	}
	return result
}

// AsSecrets returns files as base64-encoded secrets.
func (f *Files) AsSecrets() map[string]string {
	result := make(map[string]string)
	for name, data := range f.files {
		// In a real implementation, this would base64 encode
		result[path.Base(name)] = string(data)
	}
	return result
}

// Lines returns file content as a slice of lines.
func (f *Files) Lines(name string) []string {
	if data, ok := f.files[name]; ok {
		return strings.Split(string(data), "\n")
	}
	return nil
}

//go:wasmexport helm_plugin_main
func HelmPluginMain() uint32 {
	pdk.Log(pdk.LogDebug, "gotemplate-render plugin starting")

	// Read input from Extism
	inputBytes := pdk.Input()

	// Parse the input message
	var input InputMessageRenderV1
	if err := json.Unmarshal(inputBytes, &input); err != nil {
		return outputError(fmt.Sprintf("failed to parse input: %v", err))
	}

	pdk.Log(pdk.LogDebug, fmt.Sprintf("Received %d source files", len(input.SourceFiles)))

	output := OutputMessageRenderV1{
		RenderedFiles: make(map[string]string),
	}

	// Build files map for template access
	filesMap := make(map[string][]byte)
	for _, f := range input.Files {
		filesMap[f.Name] = f.Data
	}
	files := &Files{files: filesMap}

	// Create a master template for includes
	masterTmpl := template.New("gotpl")
	masterTmpl.Funcs(funcMap())

	// First pass: parse all templates to enable includes
	for _, file := range input.SourceFiles {
		if strings.HasPrefix(path.Base(file.Name), "_") {
			// Partial templates (helpers)
			_, err := masterTmpl.New(file.Name).Parse(string(file.Data))
			if err != nil {
				output.Errors = append(output.Errors,
					fmt.Sprintf("parse error in %s: %v", file.Name, err))
			}
		}
	}

	// Second pass: parse and render regular templates
	for _, file := range input.SourceFiles {
		baseName := path.Base(file.Name)

		// Skip partials (they're already parsed)
		if strings.HasPrefix(baseName, "_") {
			continue
		}

		// Skip non-template files
		if !strings.HasSuffix(file.Name, ".yaml") &&
			!strings.HasSuffix(file.Name, ".yml") &&
			!strings.HasSuffix(file.Name, ".tpl") &&
			!strings.HasSuffix(file.Name, ".txt") {
			continue
		}

		pdk.Log(pdk.LogDebug, fmt.Sprintf("Rendering template: %s", file.Name))

		// Build template data
		data := TemplateData{
			Release:      input.Release,
			Values:       input.Values,
			Chart:        input.Chart,
			Subcharts:    input.Subcharts,
			Files:        files,
			Capabilities: input.Capabilities,
			Template: TemplateInfo{
				Name:     file.Name,
				BasePath: path.Dir(file.Name),
			},
		}

		// Parse the template
		tmpl, err := masterTmpl.Clone()
		if err != nil {
			output.Errors = append(output.Errors,
				fmt.Sprintf("clone error for %s: %v", file.Name, err))
			continue
		}

		tmpl, err = tmpl.New(file.Name).Parse(string(file.Data))
		if err != nil {
			output.Errors = append(output.Errors,
				fmt.Sprintf("parse error in %s: %v", file.Name, err))
			continue
		}

		// Execute the template
		var buf bytes.Buffer
		if err := tmpl.Execute(&buf, data); err != nil {
			output.Errors = append(output.Errors,
				fmt.Sprintf("render error in %s: %v", file.Name, err))
			continue
		}

		rendered := buf.String()

		// Skip empty output
		if strings.TrimSpace(rendered) == "" {
			continue
		}

		output.RenderedFiles[file.Name] = rendered
	}

	// Marshal and return the output
	outputBytes, err := json.Marshal(output)
	if err != nil {
		return outputError(fmt.Sprintf("failed to marshal output: %v", err))
	}

	pdk.Output(outputBytes)
	pdk.Log(pdk.LogDebug, "gotemplate-render plugin completed successfully")
	return 0
}

// funcMap returns the template functions available in gotemplate.
// This is a simplified set - a full implementation would include all Sprig functions.
func funcMap() template.FuncMap {
	return template.FuncMap{
		// String functions
		"upper":      strings.ToUpper,
		"lower":      strings.ToLower,
		"title":      strings.Title,
		"trim":       strings.TrimSpace,
		"trimPrefix": strings.TrimPrefix,
		"trimSuffix": strings.TrimSuffix,
		"contains":   strings.Contains,
		"hasPrefix":  strings.HasPrefix,
		"hasSuffix":  strings.HasSuffix,
		"replace":    strings.ReplaceAll,
		"repeat":     strings.Repeat,
		"join":       strings.Join,
		"split":      strings.Split,

		// Default values
		"default": func(def interface{}, val interface{}) interface{} {
			if val == nil || val == "" {
				return def
			}
			return val
		},

		// Required value
		"required": func(msg string, val interface{}) (interface{}, error) {
			if val == nil || val == "" {
				return nil, fmt.Errorf(msg)
			}
			return val, nil
		},

		// Conditional
		"ternary": func(trueVal, falseVal interface{}, cond bool) interface{} {
			if cond {
				return trueVal
			}
			return falseVal
		},

		// Empty check
		"empty": func(val interface{}) bool {
			if val == nil {
				return true
			}
			switch v := val.(type) {
			case string:
				return v == ""
			case []interface{}:
				return len(v) == 0
			case map[string]interface{}:
				return len(v) == 0
			}
			return false
		},

		// Coalesce returns first non-empty value
		"coalesce": func(vals ...interface{}) interface{} {
			for _, v := range vals {
				if v != nil && v != "" {
					return v
				}
			}
			return nil
		},

		// Quote wraps a string in quotes
		"quote": func(s string) string {
			return fmt.Sprintf("%q", s)
		},

		// Squote wraps a string in single quotes
		"squote": func(s string) string {
			return fmt.Sprintf("'%s'", s)
		},

		// Printf
		"printf": fmt.Sprintf,

		// Fail explicitly fails rendering
		"fail": func(msg string) (string, error) {
			return "", fmt.Errorf(msg)
		},

		// Include is a placeholder - actual implementation handled differently
		"include": func(name string, data interface{}) (string, error) {
			return "", fmt.Errorf("include not fully implemented in plugin")
		},

		// tpl is a placeholder
		"tpl": func(tpl string, data interface{}) (string, error) {
			return "", fmt.Errorf("tpl not fully implemented in plugin")
		},

		// toYaml converts a value to YAML
		"toYaml": func(v interface{}) string {
			// Simplified - real implementation would use yaml.Marshal
			data, err := json.MarshalIndent(v, "", "  ")
			if err != nil {
				return ""
			}
			return string(data)
		},

		// toJson converts a value to JSON
		"toJson": func(v interface{}) string {
			data, err := json.Marshal(v)
			if err != nil {
				return ""
			}
			return string(data)
		},

		// toPrettyJson converts a value to formatted JSON
		"toPrettyJson": func(v interface{}) string {
			data, err := json.MarshalIndent(v, "", "  ")
			if err != nil {
				return ""
			}
			return string(data)
		},

		// Indent adds indentation to each line
		"indent": func(spaces int, s string) string {
			prefix := strings.Repeat(" ", spaces)
			lines := strings.Split(s, "\n")
			for i, line := range lines {
				if line != "" {
					lines[i] = prefix + line
				}
			}
			return strings.Join(lines, "\n")
		},

		// Nindent is indent with a newline prefix
		"nindent": func(spaces int, s string) string {
			prefix := strings.Repeat(" ", spaces)
			lines := strings.Split(s, "\n")
			for i, line := range lines {
				if line != "" {
					lines[i] = prefix + line
				}
			}
			return "\n" + strings.Join(lines, "\n")
		},

		// List creates a list
		"list": func(items ...interface{}) []interface{} {
			return items
		},

		// Dict creates a dictionary
		"dict": func(vals ...interface{}) map[string]interface{} {
			result := make(map[string]interface{})
			for i := 0; i < len(vals)-1; i += 2 {
				key, ok := vals[i].(string)
				if ok {
					result[key] = vals[i+1]
				}
			}
			return result
		},
	}
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
