package metadata

import "fmt"

// MetadataError represents a metadata embedding error.
type MetadataError struct {
	Message  string
	Original error
}

func (e *MetadataError) Error() string {
	if e.Original != nil {
		return fmt.Sprintf("Metadata error: %s: %v", e.Message, e.Original)
	}
	return fmt.Sprintf("Metadata error: %s", e.Message)
}

func (e *MetadataError) Unwrap() error {
	return e.Original
}
