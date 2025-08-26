// Package config provides utilities for merging configuration structs.
package config

import (
	"reflect"
)

// Merger provides utilities for merging configuration structs
type Merger struct{}

// New creates a new Merger instance
func New() *Merger {
	return &Merger{}
}

// Merge merges the source config into the target config, preserving existing values
// when source fields are zero/nil values.
func (m *Merger) Merge(target, source interface{}) {
	if source == nil {
		return
	}

	targetVal := reflect.ValueOf(target)
	sourceVal := reflect.ValueOf(source)

	// Dereference pointers
	if targetVal.Kind() == reflect.Ptr {
		targetVal = targetVal.Elem()
	}
	if sourceVal.Kind() == reflect.Ptr {
		sourceVal = sourceVal.Elem()
	}

	// Only work with structs
	if targetVal.Kind() != reflect.Struct || sourceVal.Kind() != reflect.Struct {
		return
	}

	// Iterate through all fields in the source
	for i := 0; i < sourceVal.NumField(); i++ {
		sourceField := sourceVal.Field(i)
		sourceFieldType := sourceVal.Type().Field(i)

		// Find corresponding field in target
		targetField := targetVal.FieldByName(sourceFieldType.Name)
		if !targetField.IsValid() || !targetField.CanSet() {
			continue
		}

		// Only merge if source field is not zero/nil
		switch sourceField.Kind() {
		case reflect.Slice, reflect.Map:
			// treat empty as zero
			if !sourceField.IsNil() && sourceField.Len() > 0 {
				targetField.Set(sourceField)
			}
		default:
			if !sourceField.IsZero() {
				targetField.Set(sourceField)
			}
		}
	}
}

// Merge is a convenience function that creates a new merger and merges configs
func Merge(target, source interface{}) {
	New().Merge(target, source)
}
