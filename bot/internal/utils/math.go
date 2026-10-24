package utils

import "math/rand"

// GetRandom returns a random number between min and max (inclusive).
func GetRandom(min, max int) int {
	return rand.Intn(max-min+1) + min
}

// RatedChoice is true once every rate calls on average; 0 never.
func RatedChoice(rate int) bool {
	return rate == 1 || (rate > 1 && GetRandom(1, rate) == 1)
}
