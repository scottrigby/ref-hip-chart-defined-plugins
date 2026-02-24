// Package main demonstrates basic SDK usage for rendering charts with plugins.
//
// This example uses the default disk-based caches:
//   - Plugin tarballs: $HELM_CACHE_HOME/content/{digest}.plugin
//   - Wasm compilation: $HELM_CACHE_HOME/wazero-build/
//
// Suitable for environments with writable filesystems.
package main

import (
	"context"
	"fmt"
	"log"
	"os"

	"helm.sh/helm/v4/pkg/chart"
	"helm.sh/helm/v4/pkg/chart/loader"
	"helm.sh/helm/v4/pkg/helmpath"
	"helm.sh/helm/v4/pkg/render"
)

func main() {
	if len(os.Args) < 2 {
		log.Fatal("Usage: simple <chart-path>")
	}
	chartPath := os.Args[1]

	// Load the chart (auto-detects v2 vs v3 API version)
	chrt, err := loader.Load(chartPath)
	if err != nil {
		log.Fatalf("Failed to load chart: %v", err)
	}

	// Create accessor to get chart values
	accessor, err := chart.NewAccessor(chrt)
	if err != nil {
		log.Fatalf("Failed to create accessor: %v", err)
	}

	// Create a plugin renderer with default settings
	// This uses disk-based caches at $HELM_CACHE_HOME
	renderer := &render.PluginRenderer{
		ContentCachePath: helmpath.CachePath("content"),
		// CompilationCache: nil means use default disk cache
		// PreloadedPlugins: nil means load from disk
	}

	// Build render context with release info and values
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
