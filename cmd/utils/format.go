package utils

import (
	"fmt"
	"math"
	"reflect"
	"strings"
	"unsafe"
)

// FormatBytes converts a size in bytes to a human-readable format.
func FormatBytes(bytes uint64) string {
	sizes := []string{"Bytes", "KB", "MB", "GB", "TB"}

	if bytes == 0 {
		return "0 Byte"
	}

	i := math.Floor(math.Log2(float64(bytes)) / 10)
	formattedSize := float64(bytes) / math.Pow(1024, i)
	return fmt.Sprintf("%.2f %s", formattedSize, sizes[int(i)])
}

// FormatNumber formats a number with thousands separators (using a single quote as the separator).
func FormatNumber(number float64) string {
	parts := strings.Split(fmt.Sprintf("%.2f", number), ".")
	integerPart := parts[0]
	decimalPart := parts[1]

	var result strings.Builder
	length := len(integerPart)

	for idx, char := range integerPart {
		if idx > 0 && (length-idx)%3 == 0 {
			result.WriteRune('\'')
		}
		result.WriteRune(char)
	}

	if len(parts) > 1 && decimalPart != "00" {
		return result.String() + "." + decimalPart
	}

	return result.String()
}

// MeasureSize calculates the memory usage of an object in bytes
func MeasureSize(obj interface{}) uintptr {
	visited := make(map[uintptr]bool)
	return deepSize(reflect.ValueOf(obj), visited)
}

func deepSize(v reflect.Value, visited map[uintptr]bool) uintptr {
	if !v.IsValid() {
		return 0
	}

	switch v.Kind() {
	case reflect.Ptr, reflect.Interface:
		ptr := v.Pointer()
		if ptr != 0 && !visited[ptr] {
			visited[ptr] = true
			return uintptr(unsafe.Sizeof(ptr)) + deepSize(v.Elem(), visited)
		}
		return uintptr(unsafe.Sizeof(ptr))
	case reflect.Array, reflect.Slice:
		size := uintptr(0)
		for i := 0; i < v.Len(); i++ {
			size += deepSize(v.Index(i), visited)
		}
		return size + uintptr(v.Cap())*unsafe.Sizeof(uintptr(0))
	case reflect.Map:
		size := uintptr(0)
		for _, key := range v.MapKeys() {
			size += deepSize(key, visited) + deepSize(v.MapIndex(key), visited)
		}
		return size + uintptr(v.Len())*unsafe.Sizeof(uintptr(0))
	case reflect.Struct:
		size := uintptr(0)
		for i := 0; i < v.NumField(); i++ {
			size += deepSize(v.Field(i), visited)
		}
		return size
	default:
		return uintptr(unsafe.Sizeof(v.Interface()))
	}
}
