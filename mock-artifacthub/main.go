// mock-artifacthub provides a mock ArtifactHub API server for testing
// chart-defined plugin discovery. It dynamically discovers plugins from
// the configured OCI registry.
package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"log"
	"net/http"
	"os"
	"os/exec"
	"regexp"
	"strconv"
	"strings"
)

// RepositoryKind represents the type of repository in ArtifactHub.
type RepositoryKind int

const (
	KindHelm       RepositoryKind = 0
	KindHelmPlugin RepositoryKind = 6
)

// SignKey represents signing key information.
type SignKey struct {
	Fingerprint string `json:"fingerprint,omitempty"`
	URL         string `json:"url,omitempty"`
}

// Repository represents an ArtifactHub repository.
type Repository struct {
	RepositoryID      string         `json:"repository_id"`
	Kind              RepositoryKind `json:"kind"`
	Name              string         `json:"name"`
	DisplayName       string         `json:"display_name,omitempty"`
	URL               string         `json:"url"`
	VerifiedPublisher bool           `json:"verified_publisher"`
	Official          bool           `json:"official"`
}

// PluginData contains Helm plugin-specific metadata.
type PluginData struct {
	PluginType            string   `json:"pluginType,omitempty"`
	Runtime               string   `json:"runtime,omitempty"`
	HelmVersionConstraint string   `json:"helmVersionConstraint,omitempty"`
	Platforms             []string `json:"platforms,omitempty"`
	FilePatterns          []string `json:"filePatterns,omitempty"`
}

// PluginPackage represents a Helm plugin package in ArtifactHub.
type PluginPackage struct {
	PackageID   string      `json:"package_id"`
	Name        string      `json:"name"`
	DisplayName string      `json:"display_name,omitempty"`
	Description string      `json:"description,omitempty"`
	Version     string      `json:"version"`
	License     string      `json:"license,omitempty"`
	Signed      bool        `json:"signed"`
	Signatures  []string    `json:"signatures,omitempty"`
	SignKey     *SignKey    `json:"sign_key,omitempty"`
	ContentURL  string      `json:"content_url"`
	TS          int64       `json:"ts,omitempty"`
	Data        *PluginData `json:"data,omitempty"`
	Repository  *Repository `json:"repository,omitempty"`
	Keywords    []string    `json:"keywords,omitempty"`
}

// Config holds server configuration.
type Config struct {
	Port       int
	Registry   string // e.g., "ghcr.io/scottrigby/ref-hip-chart-defined-plugins"
	RepoName   string // Repository name for ArtifactHub
	RepoID     string // Repository ID
	SigningKey string // URL to signing key (optional)
	PluginsDir string // Local plugins directory for fallback discovery
}

// Server handles mock ArtifactHub API requests.
type Server struct {
	config   Config
	plugins  map[string][]PluginPackage // name -> versions
	registry *Repository
}

// NewServer creates a new mock server.
func NewServer(cfg Config) *Server {
	return &Server{
		config:  cfg,
		plugins: make(map[string][]PluginPackage),
		registry: &Repository{
			RepositoryID:      cfg.RepoID,
			Kind:              KindHelmPlugin,
			Name:              cfg.RepoName,
			DisplayName:       "Chart-Defined Plugins Reference",
			URL:               fmt.Sprintf("oci://%s/plugins", cfg.Registry),
			VerifiedPublisher: false,
			Official:          false,
		},
	}
}

// discoverPlugins discovers available plugins from the OCI registry.
func (s *Server) discoverPlugins() error {
	log.Printf("Discovering plugins from %s/plugins...", s.config.Registry)

	// GHCR doesn't support the catalog API, so we discover plugin names from
	// local directory and then fetch versions from the registry
	if strings.HasPrefix(s.config.Registry, "ghcr.io") {
		return s.discoverPluginsFromLocal()
	}

	// For other registries, try oras repo ls
	cmd := exec.Command("oras", "repo", "ls", fmt.Sprintf("%s/plugins", s.config.Registry))
	output, err := cmd.CombinedOutput()
	if err != nil {
		log.Printf("OCI discovery failed (oras repo ls): %v", err)
		if len(output) > 0 {
			log.Printf("oras output: %s", string(output))
		}
		// Fallback: try to discover from local plugins directory
		return s.discoverPluginsFromLocal()
	}

	repos := strings.Split(strings.TrimSpace(string(output)), "\n")
	for _, repo := range repos {
		if repo == "" {
			continue
		}

		pluginName := strings.TrimPrefix(repo, "plugins/")
		if err := s.discoverPluginVersions(pluginName); err != nil {
			log.Printf("Warning: failed to discover versions for %s: %v", pluginName, err)
		}
	}

	return nil
}

// discoverPluginsFromLocal discovers plugin names from local directory,
// then fetches versions from the OCI registry if GITHUB_TOKEN is available.
func (s *Server) discoverPluginsFromLocal() error {
	pluginsDir := s.config.PluginsDir
	log.Printf("Discovering plugin names from: %s", pluginsDir)

	entries, err := os.ReadDir(pluginsDir)
	if err != nil {
		return fmt.Errorf("failed to read plugins directory %s: %w", pluginsDir, err)
	}

	token := os.Getenv("GITHUB_TOKEN")
	useOCI := token != "" && strings.HasPrefix(s.config.Registry, "ghcr.io")

	if useOCI {
		log.Println("Using GITHUB_TOKEN to fetch versions from GHCR")
	}

	for _, entry := range entries {
		if !entry.IsDir() {
			continue
		}

		pluginName := entry.Name()

		// Try to get versions from OCI registry
		if useOCI {
			if err := s.discoverPluginVersions(pluginName); err != nil {
				log.Printf("Warning: failed to discover OCI versions for %s: %v, using local", pluginName, err)
				s.addLocalPlugin(pluginName)
			}
		} else {
			s.addLocalPlugin(pluginName)
		}
	}

	return nil
}

// discoverPluginVersions discovers all versions of a plugin.
func (s *Server) discoverPluginVersions(pluginName string) error {
	ref := fmt.Sprintf("%s/plugins/%s", s.config.Registry, pluginName)

	// Use oras to list tags
	var cmd *exec.Cmd
	if token := os.Getenv("GITHUB_TOKEN"); token != "" && strings.HasPrefix(s.config.Registry, "ghcr.io") {
		cmd = exec.Command("oras", "repo", "tags",
			"--username", "_",
			"--password-stdin",
			ref)
		cmd.Stdin = strings.NewReader(token)
	} else {
		cmd = exec.Command("oras", "repo", "tags", ref)
	}
	output, err := cmd.Output()
	if err != nil {
		return fmt.Errorf("failed to list tags for %s: %w", ref, err)
	}

	tags := strings.Split(strings.TrimSpace(string(output)), "\n")
	for _, tag := range tags {
		if tag == "" || tag == "latest" {
			continue
		}

		pkg := PluginPackage{
			PackageID:   fmt.Sprintf("%s-%s", pluginName, tag),
			Name:        pluginName,
			DisplayName: formatDisplayName(pluginName),
			Description: fmt.Sprintf("Helm 4 render plugin: %s", pluginName),
			Version:     tag,
			License:     "Apache-2.0",
			Signed:      s.config.SigningKey != "",
			ContentURL:  fmt.Sprintf("oci://%s/plugins/%s:%s", s.config.Registry, pluginName, tag),
			Repository:  s.registry,
			Data: &PluginData{
				PluginType:            "render/v1",
				Runtime:               "wasm",
				HelmVersionConstraint: ">=4.0.0",
			},
			Keywords: []string{"helm", "helm-plugin", "helm4", "render", "wasm"},
		}

		if s.config.SigningKey != "" {
			pkg.SignKey = &SignKey{
				URL: s.config.SigningKey,
			}
		}

		s.plugins[pluginName] = append(s.plugins[pluginName], pkg)
	}

	log.Printf("Discovered %d versions of %s", len(s.plugins[pluginName]), pluginName)
	return nil
}

// addLocalPlugin adds a plugin by reading its metadata from local plugin.yaml.
func (s *Server) addLocalPlugin(pluginName string) {
	pluginYaml := fmt.Sprintf("%s/%s/plugin.yaml", s.config.PluginsDir, pluginName)

	data, err := os.ReadFile(pluginYaml)
	if err != nil {
		log.Printf("Warning: no plugin.yaml for %s: %v", pluginName, err)
		return
	}

	// Simple YAML parsing for version
	version := extractYAMLField(string(data), "version")
	if version == "" {
		version = "0.1.0"
	}

	pluginType := extractYAMLField(string(data), "type")
	if pluginType == "" {
		pluginType = "render/v1"
	}

	pkg := PluginPackage{
		PackageID:   fmt.Sprintf("%s-%s", pluginName, version),
		Name:        pluginName,
		DisplayName: formatDisplayName(pluginName),
		Description: extractYAMLField(string(data), "description"),
		Version:     version,
		License:     "Apache-2.0",
		ContentURL:  fmt.Sprintf("oci://%s/plugins/%s:%s", s.config.Registry, pluginName, version),
		Repository:  s.registry,
		Data: &PluginData{
			PluginType:            pluginType,
			Runtime:               extractYAMLField(string(data), "runtime"),
			HelmVersionConstraint: ">=4.0.0",
		},
		Keywords: []string{"helm", "helm-plugin", "helm4", pluginType},
	}

	s.plugins[pluginName] = append(s.plugins[pluginName], pkg)
	log.Printf("Discovered local plugin: %s@%s", pluginName, version)
}

// extractYAMLField extracts a simple field from YAML content.
func extractYAMLField(content, field string) string {
	re := regexp.MustCompile(fmt.Sprintf(`(?m)^%s:\s*["']?([^"'\n]+)["']?`, field))
	matches := re.FindStringSubmatch(content)
	if len(matches) > 1 {
		return strings.TrimSpace(matches[1])
	}
	return ""
}

// formatDisplayName converts plugin-name to Plugin Name.
func formatDisplayName(name string) string {
	parts := strings.Split(name, "-")
	for i, p := range parts {
		if len(p) > 0 {
			parts[i] = strings.ToUpper(p[:1]) + p[1:]
		}
	}
	return strings.Join(parts, " ")
}

// handlePlugin handles /api/v1/packages/helm-plugin/{repo}/{name}[/{version}]
func (s *Server) handlePlugin(w http.ResponseWriter, r *http.Request) {
	path := strings.TrimPrefix(r.URL.Path, "/api/v1/packages/helm-plugin/")
	parts := strings.Split(path, "/")

	if len(parts) < 2 {
		http.NotFound(w, r)
		return
	}

	pluginName := parts[1]
	versions, ok := s.plugins[pluginName]
	if !ok || len(versions) == 0 {
		http.NotFound(w, r)
		return
	}

	var pkg *PluginPackage
	if len(parts) == 3 {
		// Specific version
		version := parts[2]
		for i := range versions {
			if versions[i].Version == version {
				pkg = &versions[i]
				break
			}
		}
	} else {
		// Latest version (last in list)
		pkg = &versions[len(versions)-1]
	}

	if pkg == nil {
		http.NotFound(w, r)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(pkg)
}

// handleSearch handles /api/v1/packages/search
func (s *Server) handleSearch(w http.ResponseWriter, r *http.Request) {
	query := r.URL.Query()

	// Check kind filter
	kindStr := query.Get("kind")
	if kindStr != "" && kindStr != strconv.Itoa(int(KindHelmPlugin)) {
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]interface{}{"packages": []interface{}{}})
		return
	}

	searchQuery := strings.ToLower(query.Get("ts_query_web"))

	var results []PluginPackage
	for _, versions := range s.plugins {
		if len(versions) == 0 {
			continue
		}
		// Return latest version
		pkg := versions[len(versions)-1]

		// Apply search filter
		if searchQuery != "" {
			match := strings.Contains(strings.ToLower(pkg.Name), searchQuery) ||
				strings.Contains(strings.ToLower(pkg.Description), searchQuery)
			if !match {
				continue
			}
		}

		results = append(results, pkg)
	}

	// Apply pagination
	offset := 0
	if o := query.Get("offset"); o != "" {
		offset, _ = strconv.Atoi(o)
	}
	limit := 20
	if l := query.Get("limit"); l != "" {
		limit, _ = strconv.Atoi(l)
	}

	totalCount := len(results)
	if offset < len(results) {
		end := offset + limit
		if end > len(results) {
			end = len(results)
		}
		results = results[offset:end]
	} else {
		results = nil
	}

	w.Header().Set("Content-Type", "application/json")
	w.Header().Set("Pagination-Total-Count", strconv.Itoa(totalCount))
	json.NewEncoder(w).Encode(map[string]interface{}{"packages": results})
}

// handleHealth handles /health
func (s *Server) handleHealth(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{
		"status":       "ok",
		"plugin_count": len(s.plugins),
	})
}

func main() {
	port := flag.Int("port", 8080, "Server port")
	registry := flag.String("registry", "ghcr.io/scottrigby/ref-hip-chart-defined-plugins", "OCI registry path")
	repoName := flag.String("repo-name", "ref-hip-chart-defined-plugins", "Repository name")
	repoID := flag.String("repo-id", "ref-hip-chart-defined-plugins", "Repository ID")
	signingKey := flag.String("signing-key", "", "URL to signing key")
	pluginsDir := flag.String("plugins-dir", "../plugins", "Local plugins directory for fallback discovery")
	flag.Parse()

	cfg := Config{
		Port:       *port,
		Registry:   *registry,
		RepoName:   *repoName,
		RepoID:     *repoID,
		SigningKey: *signingKey,
		PluginsDir: *pluginsDir,
	}

	server := NewServer(cfg)

	// Discover plugins on startup
	if err := server.discoverPlugins(); err != nil {
		log.Printf("Warning: plugin discovery failed: %v", err)
	}

	// Set up routes
	http.HandleFunc("/api/v1/packages/helm-plugin/", server.handlePlugin)
	http.HandleFunc("/api/v1/packages/search", server.handleSearch)
	http.HandleFunc("/health", server.handleHealth)

	addr := fmt.Sprintf(":%d", cfg.Port)
	log.Printf("Mock ArtifactHub server starting on %s", addr)
	log.Printf("Registry: %s", cfg.Registry)
	log.Printf("Discovered %d plugins", len(server.plugins))

	if err := http.ListenAndServe(addr, nil); err != nil {
		log.Fatalf("Server failed: %v", err)
	}
}
