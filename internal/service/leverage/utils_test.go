package leverage

import (
	"reflect"
	"testing"
)

func TestMergeDailyBarsFillsMissingDaysFromIntraday(t *testing.T) {
	// 模擬 Yahoo 盤中日K：11-12 為 null 被略過，11-13 (今日) 尚未出現
	daily := []dailyBar{
		{date: "2026-11-10", close: 99.7, low: 97.8},
		{date: "2026-11-11", close: 103.2, low: 99.8},
	}
	intraday := []dailyBar{
		{date: "2026-11-11", close: 103.1, low: 101.0},
		{date: "2026-11-12", close: 104.05, low: 102.4},
		{date: "2026-11-12", close: 101.5, low: 101.5},
		{date: "2026-11-12", close: 100.85, low: 100.8},
		{date: "2026-11-13", close: 104.15, low: 102.85},
		{date: "2026-11-13", close: 103.1, low: 103.05},
	}

	got := mergeDailyBars(daily, aggregateIntradayBars(intraday))
	want := []dailyBar{
		{date: "2026-11-10", close: 99.7, low: 97.8},
		{date: "2026-11-11", close: 103.2, low: 99.8},
		{date: "2026-11-12", close: 100.85, low: 100.8},
		{date: "2026-11-13", close: 103.1, low: 102.85},
	}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("got %+v, want %+v", got, want)
	}
}
