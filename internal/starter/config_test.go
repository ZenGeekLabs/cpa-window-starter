package starter

import (
	"net/http"
	"testing"
	"time"
)

func TestEditableTimesAndTimezone(t *testing.T) {
	cfg, err := ParseConfig([]byte("times: ['20:15', '08:30']\ntimezone: Asia/Shanghai\naccounts: ['a']\n"))
	if err != nil {
		t.Fatal(err)
	}
	if cfg.Times[0] != "08:30" || cfg.Times[1] != "20:15" {
		t.Fatal("custom times were not preserved and sorted")
	}
	now := time.Date(2030, 10, 2, 0, 0, 0, 0, time.UTC)
	next, err := NextOccurrence(now, cfg)
	if err != nil || next.UTC().Hour() != 0 || next.UTC().Minute() != 30 {
		t.Fatalf("wrong timezone conversion: %v %v", next, err)
	}
	next, _ = NextOccurrence(time.Date(2030, 10, 2, 21, 0, 0, 0, next.Location()), cfg)
	if next.Day() != 3 || next.Hour() != 8 || next.Minute() != 30 {
		t.Fatal("next-day trigger is incorrect")
	}
}

func TestRejectInvalidSchedules(t *testing.T) {
	for _, times := range [][]string{{"24:00"}, {"7:00"}, {"13:00", "13:00"}, {}} {
		cfg := DefaultConfig()
		cfg.Times = times
		if _, err := ValidateConfig(cfg); err == nil {
			t.Fatalf("accepted invalid times %v", times)
		}
	}
	cfg := DefaultConfig()
	cfg.Timezone = "not/a-zone"
	if _, err := ValidateConfig(cfg); err == nil {
		t.Fatal("invalid timezone accepted")
	}
}

func TestActualResetUsesFiveHourBucketOnly(t *testing.T) {
	observed := time.Date(2030, 10, 2, 7, 0, 0, 0, time.UTC)
	weekly := http.Header{"X-Codex-Primary-Window-Minutes": {"10080"}, "X-Codex-Primary-Reset-At": {"1917172800"}}
	if reset, _ := FiveHourQuota(weekly, observed); reset != nil {
		t.Fatal("weekly reset shown as five-hour reset")
	}
	weekly["x-codex-secondary-window-minutes"] = []string{"300"}
	weekly["x-codex-secondary-reset-after-seconds"] = []string{"18000"}
	weekly["x-codex-secondary-used-percent"] = []string{"0.5"}
	reset, used := FiveHourQuota(weekly, observed)
	if reset == nil || !reset.Equal(observed.Add(5*time.Hour)) || used == nil || *used != 0.5 {
		t.Fatal("five-hour bucket not parsed")
	}
	missing := http.Header{"X-Codex-Primary-Reset-At": {"1917172800"}}
	if reset, _ := FiveHourQuota(missing, observed); reset != nil {
		t.Fatal("unidentified quota window treated as five hours")
	}
}
