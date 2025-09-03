package utils

import (
	"testing"
)

func TestIntPtr(t *testing.T) {
	value := 42
	ptr := IntPtr(value)

	if ptr == nil {
		t.Fatal("Expected non-nil pointer")
	}

	if *ptr != value {
		t.Errorf("Expected pointer to contain %d, got %d", value, *ptr)
	}
}

func TestInt32Ptr(t *testing.T) {
	value := int32(42)
	ptr := Int32Ptr(value)

	if ptr == nil {
		t.Fatal("Expected non-nil pointer")
	}

	if *ptr != value {
		t.Errorf("Expected pointer to contain %d, got %d", value, *ptr)
	}
}

func TestInt64Ptr(t *testing.T) {
	value := int64(42)
	ptr := Int64Ptr(value)

	if ptr == nil {
		t.Fatal("Expected non-nil pointer")
	}

	if *ptr != value {
		t.Errorf("Expected pointer to contain %d, got %d", value, *ptr)
	}
}

func TestUintPtr(t *testing.T) {
	value := uint(42)
	ptr := UintPtr(value)

	if ptr == nil {
		t.Fatal("Expected non-nil pointer")
	}

	if *ptr != value {
		t.Errorf("Expected pointer to contain %d, got %d", value, *ptr)
	}
}

func TestUint32Ptr(t *testing.T) {
	value := uint32(42)
	ptr := Uint32Ptr(value)

	if ptr == nil {
		t.Fatal("Expected non-nil pointer")
	}

	if *ptr != value {
		t.Errorf("Expected pointer to contain %d, got %d", value, *ptr)
	}
}

func TestUint64Ptr(t *testing.T) {
	value := uint64(42)
	ptr := Uint64Ptr(value)

	if ptr == nil {
		t.Fatal("Expected non-nil pointer")
	}

	if *ptr != value {
		t.Errorf("Expected pointer to contain %d, got %d", value, *ptr)
	}
}

func TestFloat32Ptr(t *testing.T) {
	value := float32(3.14)
	ptr := Float32Ptr(value)

	if ptr == nil {
		t.Fatal("Expected non-nil pointer")
	}

	if *ptr != value {
		t.Errorf("Expected pointer to contain %f, got %f", value, *ptr)
	}
}

func TestFloat64Ptr(t *testing.T) {
	value := 3.14
	ptr := Float64Ptr(value)

	if ptr == nil {
		t.Fatal("Expected non-nil pointer")
	}

	if *ptr != value {
		t.Errorf("Expected pointer to contain %f, got %f", value, *ptr)
	}
}

func TestBoolPtr(t *testing.T) {
	value := true
	ptr := BoolPtr(value)

	if ptr == nil {
		t.Fatal("Expected non-nil pointer")
	}

	if *ptr != value {
		t.Errorf("Expected pointer to contain %t, got %t", value, *ptr)
	}
}

func TestStringPtr(t *testing.T) {
	value := "test"
	ptr := StringPtr(value)

	if ptr == nil {
		t.Fatal("Expected non-nil pointer")
	}

	if *ptr != value {
		t.Errorf("Expected pointer to contain %s, got %s", value, *ptr)
	}
}

func TestPointerModification(t *testing.T) {
	// Test that modifying the pointer doesn't affect the original value
	original := 42
	ptr := IntPtr(original)

	*ptr = 100

	if original != 42 {
		t.Error("Modifying pointer should not affect original value")
	}

	if *ptr != 100 {
		t.Error("Pointer should contain modified value")
	}
}
