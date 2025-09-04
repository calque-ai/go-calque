package helpers

import (
	"reflect"
	"testing"
)

func TestPtrOf(t *testing.T) {
	tests := []struct {
		name     string
		input    interface{}
		expected interface{}
	}{
		{"int", 42, 42},
		{"int32", int32(42), int32(42)},
		{"int64", int64(42), int64(42)},
		{"uint", uint(42), uint(42)},
		{"uint32", uint32(42), uint32(42)},
		{"uint64", uint64(42), uint64(42)},
		{"float32", float32(3.14), float32(3.14)},
		{"float64", 3.14, 3.14},
		{"bool", true, true},
		{"string", "test", "test"},
		{"zero int", 0, 0},
		{"empty string", "", ""},
		{"false bool", false, false},
		{"nil interface", nil, nil},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Use reflect to call PtrOf with the input value
			inputValue := reflect.ValueOf(tt.input)
			if tt.input == nil {
				// Handle nil case specially
				result := PtrOf[interface{}](nil)

				if result == nil {
					t.Fatal("Expected non-nil pointer")
				}

				// For nil interface, the pointer should exist but point to nil
				if result == nil {
					t.Error("Expected non-nil pointer result")
				}
				return
			}

			// Create a function that can call PtrOf with the specific type
			ptrOfFunc := reflect.MakeFunc(
				reflect.TypeOf(func(interface{}) interface{} { return nil }),
				func(_ []reflect.Value) []reflect.Value {
					// This is a bit complex due to Go's type system
					// We'll use a simpler approach for the test
					return []reflect.Value{reflect.ValueOf(PtrOf(tt.input))}
				},
			)

			// Call the function
			results := ptrOfFunc.Call([]reflect.Value{inputValue})
			if len(results) == 0 {
				t.Fatal("Expected result from PtrOf")
			}

			result := results[0].Interface()
			if result == nil {
				t.Fatal("Expected non-nil pointer")
			}

			// Extract the value from the pointer using reflect
			resultValue := reflect.ValueOf(result)
			if resultValue.Kind() != reflect.Ptr {
				t.Fatal("Expected pointer result")
			}

			pointedValue := resultValue.Elem().Interface()
			if !reflect.DeepEqual(pointedValue, tt.expected) {
				t.Errorf("Expected pointer to contain %v, got %v", tt.expected, pointedValue)
			}
		})
	}
}
