package templates

import (
	"bytes"
	"fmt"
	"os"
	"path/filepath"
	"text/template"

	"github.com/nvidia/k8s-launch-kit/pkg/clusterconfig"
)

// templateFuncs provides helper functions for Go templates
var templateFuncs = template.FuncMap{
	"add": func(a, b int) int { return a + b },
	"sub": func(a, b int) int { return a - b },
	"len": func(s []string) int { return len(s) },
	"gt":  func(a, b int) bool { return a > b },
}

// ProcessTemplate processes a Go template file with the given config
func ProcessTemplate(templatePath string, config clusterconfig.ClusterConfig) (string, error) {
	// Read the template file
	templateContent, err := os.ReadFile(templatePath)
	if err != nil {
		return "", fmt.Errorf("failed to read template file %s: %w", templatePath, err)
	}

	// Parse the template with helper functions
	tmpl, err := template.New(filepath.Base(templatePath)).Funcs(templateFuncs).Parse(string(templateContent))
	if err != nil {
		return "", fmt.Errorf("failed to parse template %s: %w", templatePath, err)
	}

	// Execute the template
	var buf bytes.Buffer
	err = tmpl.Execute(&buf, config)
	if err != nil {
		return "", fmt.Errorf("failed to execute template %s: %w", templatePath, err)
	}

	return buf.String(), nil
}

// ProcessProfileTemplates processes all template files in a profile directory
func ProcessProfileTemplates(profileDir string, config clusterconfig.ClusterConfig) (map[string]string, error) {
	results := make(map[string]string)

	// Find all YAML files in the profile directory
	entries, err := os.ReadDir(profileDir)
	if err != nil {
		return nil, fmt.Errorf("failed to read profile directory %s: %w", profileDir, err)
	}

	for _, entry := range entries {
		if entry.IsDir() {
			continue
		}

		filename := entry.Name()
		if filepath.Ext(filename) == ".yaml" || filepath.Ext(filename) == ".yml" {
			templatePath := filepath.Join(profileDir, filename)

			processed, err := ProcessTemplate(templatePath, config)
			if err != nil {
				return nil, fmt.Errorf("failed to process template %s: %w", filename, err)
			}

			results[filename] = processed
		}
	}

	return results, nil
}
