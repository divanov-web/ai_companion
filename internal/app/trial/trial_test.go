package trial

import (
	"testing"
	"time"
)

func TestParseTrialEnd_UsesProvidedTrialDateEnd(t *testing.T) {
	originalTrialEnd := TrialDateEndRFC3339
	originalModTimeFn := executableModTimeUTC
	t.Cleanup(func() {
		TrialDateEndRFC3339 = originalTrialEnd
		executableModTimeUTC = originalModTimeFn
	})

	provided := time.Date(2026, 3, 6, 18, 54, 0, 0, time.UTC)
	TrialDateEndRFC3339 = provided.Format(time.RFC3339)

	executableModTimeUTC = func() (time.Time, error) {
		return time.Time{}, nil
	}

	got, ok := parseTrialEnd()
	if !ok {
		t.Fatalf("parseTrialEnd() returned ok=false")
	}
	if !got.Equal(provided) {
		t.Fatalf("parseTrialEnd() = %s, want %s", got.Format(time.RFC3339), provided.Format(time.RFC3339))
	}
}

func TestParseTrialEnd_FallbackIsStableAcrossRestart(t *testing.T) {
	originalTrialEnd := TrialDateEndRFC3339
	originalModTimeFn := executableModTimeUTC
	t.Cleanup(func() {
		TrialDateEndRFC3339 = originalTrialEnd
		executableModTimeUTC = originalModTimeFn
	})

	buildTime := time.Date(2026, 3, 6, 18, 54, 0, 0, time.UTC)
	executableModTimeUTC = func() (time.Time, error) {
		return buildTime, nil
	}

	TrialDateEndRFC3339 = ""
	first, ok := parseTrialEnd()
	if !ok {
		t.Fatalf("first parseTrialEnd() returned ok=false")
	}

	firstStored := TrialDateEndRFC3339
	TrialDateEndRFC3339 = ""
	second, ok := parseTrialEnd()
	if !ok {
		t.Fatalf("second parseTrialEnd() returned ok=false")
	}

	if !first.Equal(second) {
		t.Fatalf("trial end differs between runs: first=%s second=%s", first.Format(time.RFC3339), second.Format(time.RFC3339))
	}

	if TrialDateEndRFC3339 != firstStored {
		t.Fatalf("stored TrialDateEndRFC3339 changed: first=%s second=%s", firstStored, TrialDateEndRFC3339)
	}

	want := buildTime.Add(2 * time.Minute)
	if !first.Equal(want) {
		t.Fatalf("fallback trial end = %s, want %s", first.Format(time.RFC3339), want.Format(time.RFC3339))
	}
}

func TestFormatTrialEndForUser_UsesLocalTimezone(t *testing.T) {
	originalLocal := time.Local
	t.Cleanup(func() {
		time.Local = originalLocal
	})

	time.Local = time.FixedZone("UTC+3", 3*60*60)

	trialEndUTC := time.Date(2026, 3, 6, 16, 2, 0, 0, time.UTC)
	got := formatTrialEndForUser(trialEndUTC)

	const want = "2026-03-06T19:02:00+03:00"
	if got != want {
		t.Fatalf("formatTrialEndForUser() = %s, want %s", got, want)
	}
}
