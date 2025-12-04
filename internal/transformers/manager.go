package transformers

import (
	"fmt"
	"log"
	"sync"
)

var (
	instance     *Transformer
	instanceOnce sync.Once
	instanceErr  error
)

// InitTransformer initializes the global transformer instance
func InitTransformer(mappingsPath string) error {
	instanceOnce.Do(func() {
		log.Printf("[Transformer] Initializing transformer with mappings: %s", mappingsPath)

		// Create loader
		loader := NewLoader(mappingsPath)

		// Load mappings
		if err := loader.Load(); err != nil {
			instanceErr = fmt.Errorf("failed to load mappings: %w", err)
			log.Printf("[Transformer] Error: %v", instanceErr)
			return
		}

		// Create transformer
		instance = NewTransformer(loader)
		log.Printf("[Transformer] Transformer initialized successfully")
	})

	return instanceErr
}

// GetTransformer returns the global transformer instance
func GetTransformer() (*Transformer, error) {
	if instance == nil {
		return nil, fmt.Errorf("transformer not initialized, call InitTransformer first")
	}
	if instanceErr != nil {
		return nil, instanceErr
	}
	return instance, nil
}

// IsInitialized checks if the transformer has been initialized
func IsInitialized() bool {
	return instance != nil && instanceErr == nil
}
