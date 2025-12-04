package transformers

import (
	"encoding/json"
	"fmt"
	"log"

	"github.com/blues/jsonata-go"
)

// TransformError represents an error that occurred during transformation
type TransformError struct {
	Route     string
	Direction string
	Message   string
	Err       error
}

func (e *TransformError) Error() string {
	return fmt.Sprintf("transformation error for route '%s' (%s): %s", e.Route, e.Direction, e.Message)
}

// Transformer handles JSON transformations using JSONata
type Transformer struct {
	loader *Loader
}

// NewTransformer creates a new Transformer instance
func NewTransformer(loader *Loader) *Transformer {
	return &Transformer{
		loader: loader,
	}
}

// Transform applies the transformation to the input data
func (t *Transformer) Transform(route string, direction TransformDirection, inputJSON []byte) ([]byte, error) {
	log.Printf("[Transformer] Transforming %s request for route: %s", direction, route)

	// Get the transformation template
	template, err := t.loader.GetTransformTemplate(route, direction)
	if err != nil {
		return nil, &TransformError{
			Route:     route,
			Direction: string(direction),
			Message:   "template not found",
			Err:       err,
		}
	}

	// Parse input JSON
	var inputData interface{}
	if err := json.Unmarshal(inputJSON, &inputData); err != nil {
		return nil, &TransformError{
			Route:     route,
			Direction: string(direction),
			Message:   "failed to parse input JSON",
			Err:       err,
		}
	}

	log.Printf("[Transformer] Input data parsed successfully")

	// Compile JSONata expression
	expr, err := jsonata.Compile(template)
	if err != nil {
		return nil, &TransformError{
			Route:     route,
			Direction: string(direction),
			Message:   "failed to compile transformation template",
			Err:       err,
		}
	}

	log.Printf("[Transformer] JSONata expression compiled successfully")

	// Evaluate the expression
	result, err := expr.Eval(inputData)
	if err != nil {
		return nil, &TransformError{
			Route:     route,
			Direction: string(direction),
			Message:   "failed to evaluate transformation",
			Err:       err,
		}
	}

	log.Printf("[Transformer] Transformation evaluated successfully")

	// Marshal result back to JSON
	outputJSON, err := json.Marshal(result)
	if err != nil {
		return nil, &TransformError{
			Route:     route,
			Direction: string(direction),
			Message:   "failed to marshal output JSON",
			Err:       err,
		}
	}

	log.Printf("[Transformer] Transformation completed successfully for route: %s", route)
	return outputJSON, nil
}

// TransformForward applies forward transformation (BAP -> BPP format)
func (t *Transformer) TransformForward(route string, inputJSON []byte) ([]byte, error) {
	return t.Transform(route, DirectionForward, inputJSON)
}

// TransformReverse applies reverse transformation (BPP -> BAP format)
func (t *Transformer) TransformReverse(route string, inputJSON []byte) ([]byte, error) {
	return t.Transform(route, DirectionReverse, inputJSON)
}

// HasMapping checks if a mapping exists for the given route
func (t *Transformer) HasMapping(route string) bool {
	return t.loader.HasMapping(route)
}

// CreateMappingErrorResponse creates a standardized error response for mapping errors
func CreateMappingErrorResponse(route string, err error) map[string]interface{} {
	log.Printf("[Transformer] Creating mapping error response for route: %s, error: %v", route, err)

	errorResponse := map[string]interface{}{
		"mappingError": map[string]interface{}{
			"route":   route,
			"message": "Failed to transform response",
		},
	}

	// Add detailed error information if available
	if transformErr, ok := err.(*TransformError); ok {
		errorResponse["mappingError"].(map[string]interface{})["direction"] = transformErr.Direction
		errorResponse["mappingError"].(map[string]interface{})["details"] = transformErr.Message
		if transformErr.Err != nil {
			errorResponse["mappingError"].(map[string]interface{})["error"] = transformErr.Err.Error()
		}
	} else {
		errorResponse["mappingError"].(map[string]interface{})["error"] = err.Error()
	}

	return errorResponse
}

// ValidateJSON validates that the input is valid JSON
func ValidateJSON(data []byte) error {
	var js interface{}
	if err := json.Unmarshal(data, &js); err != nil {
		return fmt.Errorf("invalid JSON: %w", err)
	}
	return nil
}
