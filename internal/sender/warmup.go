package sender

import "math"

// DayNLimit applies the §11 warmup formula:
//
//	day 1 = day1
//	day n (1 < n ≤ warmupDays) = min(cap, ceil(day1 * growth^(n-1)))
//	day > warmupDays           = dailyCap (graduated)
func DayNLimit(day, day1, cap, warmupDays, dailyCap int, growth float64) int {
	if day <= 0 {
		day = 1
	}
	if day > warmupDays {
		if dailyCap > 0 {
			return dailyCap
		}
		return cap
	}
	if day == 1 {
		return day1
	}
	v := float64(day1) * math.Pow(growth, float64(day-1))
	n := int(math.Ceil(v))
	if n > cap {
		n = cap
	}
	return n
}
