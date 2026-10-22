package utils

import (
	"fmt"
	"math"
	"strings"
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
