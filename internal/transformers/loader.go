package transformers

import (
	"fmt"
	"log"
	"os"

	"gopkg.in/yaml.v3"
)

// TransformDirection represents the direction of transformation
type TransformDirection string

const (
	DirectionForward TransformDirection = "forward"
	DirectionReverse TransformDirection = "reverse"
)

// RouteTransform contains the transformation templates for a route
type RouteTransform struct {
	Forward string `yaml:"forward"`
	Reverse string `yaml:"reverse"`
}

// MappingConfig contains all route transformations
type MappingConfig struct {
	Mappings map[string]RouteTransform `yaml:"mappings"`
}

// Loader handles loading and parsing of mapping configuration
type Loader struct {
	config     *MappingConfig
	configPath string
}

// NewLoader creates a new Loader instance
func NewLoader(configPath string) *Loader {
	return &Loader{
		configPath: configPath,
	}
}

// Load reads and parses the mapping configuration file
func (l *Loader) Load() error {
	log.Printf("[Transformer] Loading mappings from: %s", l.configPath)

	// Read the YAML file
	data, err := os.ReadFile(l.configPath)
	if err != nil {
		return fmt.Errorf("failed to read mappings file: %w", err)
	}

	// Parse YAML into config structure
	var config MappingConfig
	if err := yaml.Unmarshal(data, &config); err != nil {
		return fmt.Errorf("failed to parse mappings YAML: %w", err)
	}

	// Validate that we have mappings
	if len(config.Mappings) == 0 {
		return fmt.Errorf("no mappings found in configuration file")
	}

	l.config = &config
	log.Printf("[Transformer] Successfully loaded %d route mappings", len(config.Mappings))

	// Log available routes
	for route := range config.Mappings {
		log.Printf("[Transformer] Available mapping for route: %s", route)
	}

	return nil
}

// GetConfig returns the loaded configuration
func (l *Loader) GetConfig() *MappingConfig {
	return l.config
}

// GetRouteTransform retrieves the transformation for a specific route
func (l *Loader) GetRouteTransform(route string) (*RouteTransform, error) {
	if l.config == nil {
		return nil, fmt.Errorf("configuration not loaded")
	}

	transform, exists := l.config.Mappings[route]
	if !exists {
		return nil, fmt.Errorf("no mapping found for route: %s", route)
	}

	return &transform, nil
}

// HasMapping checks if a mapping exists for the given route
func (l *Loader) HasMapping(route string) bool {
	if l.config == nil {
		return false
	}
	_, exists := l.config.Mappings[route]
	return exists
}

// GetTransformTemplate retrieves the transformation template for a route and direction
func (l *Loader) GetTransformTemplate(route string, direction TransformDirection) (string, error) {
	transform, err := l.GetRouteTransform(route)
	if err != nil {
		return "", err
	}

	switch direction {
	case DirectionForward:
		if transform.Forward == "" {
			return "", fmt.Errorf("no forward transformation defined for route: %s", route)
		}
		return transform.Forward, nil
	case DirectionReverse:
		if transform.Reverse == "" {
			return "", fmt.Errorf("no reverse transformation defined for route: %s", route)
		}
		return transform.Reverse, nil
	default:
		return "", fmt.Errorf("invalid transformation direction: %s", direction)
	}
}
