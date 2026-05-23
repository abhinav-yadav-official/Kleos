package sender

import "testing"

func TestDayNLimitMatchesPlanTable(t *testing.T) {
	// Plan §11: day 1=5, 2=7, 3=10, 4=14, 5=20, 6=27, 7=38, 8=40 (cap), 9..21=40.
	want := map[int]int{1: 5, 2: 7, 3: 10, 4: 14, 5: 20, 6: 27, 7: 38, 8: 40, 21: 40}
	for day, w := range want {
		got := DayNLimit(day, DefaultWarmupDay1Limit, DefaultWarmupCap, DefaultWarmupDays, 100, DefaultWarmupGrowth)
		if got != w {
			t.Errorf("day %d limit = %d, want %d", day, got, w)
		}
	}
}

func TestDayNLimitGraduates(t *testing.T) {
	got := DayNLimit(DefaultWarmupDays+1, DefaultWarmupDay1Limit, DefaultWarmupCap, DefaultWarmupDays, 120, DefaultWarmupGrowth)
	if got != 120 {
		t.Errorf("graduated limit = %d, want user daily_send_cap=120", got)
	}
}

func TestDayNLimitClampsToZeroDay(t *testing.T) {
	if DayNLimit(0, 5, 40, 21, 100, 1.4) != 5 {
		t.Error("day=0 should clamp to day 1")
	}
}
