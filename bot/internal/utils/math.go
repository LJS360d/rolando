package utils

import "math/rand"

// GetRandom returns a random number between min and max (inclusive).
func GetRandom(min, max int) int {
	return rand.Intn(max-min+1) + min
}
