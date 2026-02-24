// Package main demonstrates SDK usage for non-writable filesystem environments.
//
// This example uses:
//   - In-memory Wasm compilation cache (no disk writes)
//   - Pre-loaded plugin archives (bundled with the application)
//
// Suitable for:
//   - Serverless functions (AWS Lambda, Cloud Functions)
//   - Read-only container filesystems
//   - Embedded systems with limited storage
package main

import (
	"context"
	"embed"
	"fmt"
	"log"

	"github.com/tetratelabs/wazero"

	"helm.sh/helm/v4/pkg/chart"
	"helm.sh/helm/v4/pkg/chart/loader"
	"helm.sh/helm/v4/pkg/render"
)

// Embed plugin archives at compile time.
// In a real application, these would be the .plugin tarballs
// downloaded during build and embedded into the binary.
// We embed all files in plugins/ and filter for .plugin files in code.
//
//go:embed plugins/*
var embeddedPlugins embed.FS

func main() {
	// Create an in-memory compilation cache
	// This avoids any disk writes for Wasm JIT compilation
	compilationCache := wazero.NewCompilationCache()
	defer compilationCache.Close(context.Background())

	// Pre-load plugin archives from embedded files
	preloadedPlugins, err := loadEmbeddedPlugins()
	if err != nil {
		log.Fatalf("Failed to load embedded plugins: %v", err)
	}

	// Create a plugin renderer configured for non-writable filesystem.
	// For non-writable filesystems, you MUST:
	//   1. Set ContentCachePath to "" (empty string) to disable disk access
	//   2. Provide PreloadedPlugins with embedded plugin archives keyed by digest
	renderer := &render.PluginRenderer{
		ContentCachePath: "", // Disable disk-based plugin cache

		CompilationCache: compilationCache, // In-memory Wasm compilation cache
		PreloadedPlugins: preloadedPlugins, // Embedded plugin archives
	}

	// Load chart (in a real app, this might also be embedded)
	chrt, err := loadExampleChart()
	if err != nil {
		log.Fatalf("Failed to load chart: %v", err)
	}

	// Create accessor to get chart values
	accessor, err := chart.NewAccessor(chrt)
	if err != nil {
		log.Fatalf("Failed to create accessor: %v", err)
	}

	// Build render context
	// Capabilities is nil - system will use defaults
	renderCtx := &render.Context{
		Release:      buildReleaseInfo("my-release", "default"),
		Values:       accessor.Values(),
		Capabilities: nil, // Uses default cluster capabilities
	}

	// Render the chart using plugins
	rendered, err := renderer.Render(context.Background(), chrt, renderCtx)
	if err != nil {
		log.Fatalf("Failed to render: %v", err)
	}

	// Output rendered files
	for name, content := range rendered {
		fmt.Printf("--- %s ---\n%s\n", name, content)
	}
}

// loadEmbeddedPlugins loads plugin archives from embedded files.
// Returns a map of digest -> raw tarball bytes for use with PreloadedPlugins.
// The digest is extracted from the filename (format: {digest}.plugin).
func loadEmbeddedPlugins() (map[string][]byte, error) {
	plugins := make(map[string][]byte)

	// Read embedded plugin files
	entries, err := embeddedPlugins.ReadDir("plugins")
	if err != nil {
		// No embedded plugins - return empty map
		return plugins, nil
	}

	for _, entry := range entries {
		if entry.IsDir() {
			continue
		}

		// Only process .plugin files
		name := entry.Name()
		if len(name) < 8 || name[len(name)-7:] != ".plugin" {
			continue
		}

		data, err := embeddedPlugins.ReadFile("plugins/" + name)
		if err != nil {
			return nil, fmt.Errorf("failed to read embedded plugin %s: %w", name, err)
		}

		// Extract digest from filename (format: {digest}.plugin)
		// In a real app, you'd compute or store the digest differently
		digest := name[:len(name)-7] // Remove ".plugin"
		plugins[digest] = data
	}

	return plugins, nil
}

// loadExampleChart creates an example chart for demonstration.
// In a real app, this might load an embedded chart or receive it as input.
func loadExampleChart() (chart.Charter, error) {
	// For this example, we'll use the loader which requires disk access.
	// In a real non-writable-fs scenario, you'd embed the chart too.
	return loader.Load("../../charts/varsubst-chart")
}

func buildReleaseInfo(name, namespace string) render.ReleaseInfo {
	return render.ReleaseInfo{
		Name:      name,
		Namespace: namespace,
		IsInstall: true,
		IsUpgrade: false,
		Revision:  1,
		Service:   "Helm",
	}
}
