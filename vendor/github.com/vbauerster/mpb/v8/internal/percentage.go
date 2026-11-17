package internal

import "math"

// Percentage is a helper function, to calculate percentage.
func Percentage(total, current, width int64) float64 {
	if total == 0 {
		return 0
	}
	if current >= total {
		return float64(width)
	}
	return float64(width*current) / float64(total)
}

// PercentageRound same as Percentage but with math.Round.
func PercentageRound(total, current, width int64) float64 {
	if total < 0 || current < 0 {
		return 0
	}
	return math.Round(Percentage(total, current, width))
}
