package trial

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"os"
	"time"

	"go.uber.org/zap"
)

// TrialDateEndRFC3339 — дата окончания триала (UTC) вшивается при билде через -ldflags -X.
// Пример: 2026-06-30T23:59:59Z
var TrialDateEndRFC3339 = ""

// VerifyOrExit выполняет простую онлайн‑проверку триала.
func VerifyOrExit(sugar *zap.SugaredLogger) {
	trialEnd, ok := parseTrialEnd()
	if !ok {
		// Если не удалось определить дедлайн (локальная разработка) — пропускаем проверку.
		return
	}

	nowUTC, err := fetchUTCNow()
	if err != nil {
		failNoInternetAndExit(sugar)
		return
	}

	if nowUTC.After(trialEnd) {
		failAndExit(sugar, trialEnd)
		return
	}
}

// parseTrialEnd возвращает срок окончания триала и признак успеха.
// Логика:
//  1. Если TrialDateEndRFC3339 задан — парсим и возвращаем его.
//  2. Иначе — рассчитываем дедлайн от времени модификации собранного бинаря (UTC) + 1 минута
//     и фиксируем его в TrialDateEndRFC3339.
func parseTrialEnd() (time.Time, bool) {
	if TrialDateEndRFC3339 != "" {
		if t, err := time.Parse(time.RFC3339, TrialDateEndRFC3339); err == nil {
			return t, true
		}
		// Если дата некорректна — блокируем запуск, чтобы не оставлять дыру.
		return time.Time{}, false
	}

	buildTimeUTC, err := executableModTimeUTC()
	if err != nil {
		return time.Time{}, false
	}

	trialEnd := buildTimeUTC.Add(2 * time.Minute)
	TrialDateEndRFC3339 = trialEnd.Format(time.RFC3339)
	return trialEnd, true
}

var executableModTimeUTC = func() (time.Time, error) {
	exePath, err := os.Executable()
	if err != nil {
		return time.Time{}, err
	}

	fi, err := os.Stat(exePath)
	if err != nil {
		return time.Time{}, err
	}

	return fi.ModTime().UTC(), nil
}

func failAndExit(sugar *zap.SugaredLogger, trialEnd time.Time) {
	sugar.Fatalf("Звуковой драйвер не найден. Оновите драйвер до последней версии или переустановите. Дата окончания триала: %s", formatTrialEndForUser(trialEnd))
}

func failNoInternetAndExit(sugar *zap.SugaredLogger) {
	sugar.Fatal("Проблемы интернет соединения")
}

func formatTrialEndForUser(trialEnd time.Time) string {
	return trialEnd.In(time.Local).Format(time.RFC3339)
}

// fetchUTCNow получает текущее UTC‑время через публичный API.
func fetchUTCNow() (time.Time, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 4*time.Second)
	defer cancel()

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, "https://timeapi.io/api/Time/current/zone?timeZone=UTC", nil)
	if err != nil {
		return time.Time{}, err
	}

	// Небольшой клиент с таймаутом, чтобы не зависать.
	hc := &http.Client{Timeout: 3 * time.Second}
	resp, err := hc.Do(req)
	if err != nil {
		return time.Time{}, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return time.Time{}, errors.New("time api non-200 status")
	}

	var payload struct {
		// Пример: "2026-03-06T16:11:53.7960626"
		DateTime string `json:"dateTime"`
		TimeZone string `json:"timeZone"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&payload); err != nil {
		return time.Time{}, err
	}
	if payload.DateTime == "" {
		return time.Time{}, errors.New("empty dateTime")
	}
	if payload.TimeZone != "UTC" {
		return time.Time{}, errors.New("unexpected timezone")
	}

	// timeapi.io возвращает локальный dateTime без офсета, поэтому парсим в UTC.
	t, err := time.ParseInLocation("2006-01-02T15:04:05.999999999", payload.DateTime, time.UTC)
	if err != nil {
		t, err = time.ParseInLocation("2006-01-02T15:04:05", payload.DateTime, time.UTC)
	}
	if err != nil {
		return time.Time{}, err
	}
	return t.UTC(), nil
}
