package dota

import (
	"encoding/json"
	"maps"
	"math"
	"slices"
)

// TransformToEyes преобразует сырой JSON GSI (Dota2) в компактную структуру "Eyes".
// Возвращает JSON ([]byte) готовой структуры или ошибку парсинга. При отсутствии ключевых полей —
// возвращает максимально возможный частичный результат (никаких паник).
func TransformToEyes(raw []byte) ([]byte, error) {
	var m map[string]any
	if err := json.Unmarshal(raw, &m); err != nil {
		return nil, err
	}

	provider := getMap(m, "provider")
	mp := getMap(m, "map")
	player := getMap(m, "player")
	hero := getMap(m, "hero")
	abs := getMap(m, "abilities")
	items := getMap(m, "items")

	// time/score/phase
	eyes := map[string]any{}
	eyes["ts"] = toInt64(provider["timestamp"]) // может быть 0, если нет
	eyes["phase"] = toString(mp["game_state"])  // может быть пустым

	timeObj := map[string]any{
		"game":  toInt(mp["game_time"]),
		"clock": toInt(mp["clock_time"]),
		"day":   toBool(mp["daytime"]),
	}
	eyes["time"] = timeObj

	eyes["score"] = map[string]any{
		"rad":  toInt(mp["radiant_score"]),
		"dire": toInt(mp["dire_score"]),
	}

	// player
	playerGold := map[string]any{
		"total": toInt(player["gold"]),
		"r":     toInt(player["gold_reliable"]),
		"u":     toInt(player["gold_unreliable"]),
	}
	playerObj := map[string]any{
		"name":    toString(player["name"]),
		"team":    toString(player["team_name"]),
		"kda":     []int{toInt(player["kills"]), toInt(player["deaths"]), toInt(player["assists"])},
		"lh_dn":   []int{toInt(player["last_hits"]), toInt(player["denies"])},
		"gpm_xpm": []int{toInt(player["gpm"]), toInt(player["xpm"])},
		"gold":    playerGold,
	}
	eyes["player"] = playerObj

	// hero
	posX := toFloat(hero["xpos"])
	posY := toFloat(hero["ypos"])
	pos := []int{round10(posX), round10(posY)}
	// Персенты приводим к int 0..100
	hpPct := clamp01pct(toInt(hero["health_percent"]))
	mpPct := clamp01pct(toInt(hero["mana_percent"]))

	status := map[string]any{
		"silenced":    toBool(hero["silenced"]),
		"stunned":     toBool(hero["stunned"]),
		"disarmed":    toBool(hero["disarmed"]),
		"magicimmune": toBool(hero["magicimmune"]),
		"hexed":       toBool(hero["hexed"]),
		"muted":       toBool(hero["muted"]),
		"break":       toBool(hero["break"]),
		"smoked":      toBool(hero["smoked"]),
	}
	heroObj := map[string]any{
		"name":    toString(hero["name"]),
		"id":      toInt(hero["id"]),
		"lvl":     toInt(hero["level"]),
		"alive":   toBool(hero["alive"]),
		"respawn": toInt(hero["respawn_seconds"]),
		"hp":      []int{toInt(hero["health"]), toInt(hero["max_health"]), hpPct},
		"mp":      []int{toInt(hero["mana"]), toInt(hero["max_mana"]), mpPct},
		"pos":     pos,
		"status":  status,
		"upgrades": map[string]any{
			"scepter": toBool(hero["aghanims_scepter"]),
			"shard":   toBool(hero["aghanims_shard"]),
		},
		"buyback": map[string]any{
			"cost": toInt(hero["buyback_cost"]),
			"cd":   toInt(hero["buyback_cooldown"]),
		},
	}
	eyes["hero"] = heroObj

	// abilities -> sorted array with filters
	abArr := make([]map[string]any, 0)
	if len(abs) > 0 {
		keys := slices.Sorted(maps.Keys(abs))
		for _, k := range keys {
			v := abs[k]
			ab := asMap(v)
			name := toString(ab["name"])
			if name == "" || name == "empty" {
				continue
			}
			lvl := toInt(ab["level"])
			maxcd := roundCD(toFloat(ab["max_cooldown"]))
			can := toBool(ab["can_cast"])
			ult := toBool(ab["ultimate"])
			cd := roundCD(toFloat(ab["cooldown"]))
			// Фильтр невыученных/пустых
			if lvl == 0 && maxcd == 0 && !can {
				continue
			}
			abArr = append(abArr, map[string]any{
				"n":   name,
				"lvl": lvl,
				"cd":  cd,
				"max": maxcd,
				"can": can,
				"ult": ult,
			})
		}
	}
	eyes["abilities"] = abArr

	// items: inventory slot0..slot5 and teleport0
	inv := make([]map[string]any, 0, 6)
	for i := 0; i < 6; i++ {
		k := "slot" + itoa(i)
		mv := asMap(items[k])
		name := toString(mv["name"])
		if name == "" || name == "empty" {
			continue
		}
		inv = append(inv, map[string]any{
			"n":  name,
			"cd": roundCD(toFloat(mv["cooldown"])),
			"ch": firstNonZero(toInt(mv["charges"]), toInt(mv["item_charges"])),
		})
	}
	tp := map[string]any{}
	if tpv := asMap(items["teleport0"]); len(tpv) > 0 {
		tp["cd"] = roundCD(toFloat(tpv["cooldown"]))
		tp["ch"] = firstNonZero(toInt(tpv["charges"]), toInt(tpv["item_charges"]))
	}
	eyes["items"] = map[string]any{"inv": inv, "tp": tp}

	// signals
	lowHP := hpPct <= 30
	lowMP := mpPct <= 20
	tpReady := false
	if len(tp) > 0 {
		cd := toInt(tp["cd"])
		ch := toInt(tp["ch"])
		tpReady = (cd == 0 && ch > 0)
	}
	ultReady := false
	for _, a := range abArr {
		if toBool(a["ult"]) && toInt(a["lvl"]) > 0 && toInt(a["cd"]) == 0 && toBool(a["can"]) {
			ultReady = true
			break
		}
	}
	eyes["signals"] = map[string]any{
		"low_hp":    lowHP,
		"low_mana":  lowMP,
		"tp_ready":  tpReady,
		"ult_ready": ultReady,
	}

	// Готово — сериализуем в компактный JSON
	out, err := json.Marshal(eyes)
	if err != nil {
		return nil, err
	}
	return out, nil
}

// ===== helpers =====

func getMap(m map[string]any, key string) map[string]any {
	if m == nil {
		return map[string]any{}
	}
	return asMap(m[key])
}

func asMap(v any) map[string]any {
	if v == nil {
		return map[string]any{}
	}
	if mm, ok := v.(map[string]any); ok {
		return mm
	}
	return map[string]any{}
}

func toString(v any) string {
	if v == nil {
		return ""
	}
	if s, ok := v.(string); ok {
		return s
	}
	return ""
}

func toInt(v any) int {
	switch x := v.(type) {
	case float64:
		return int(math.Round(x))
	case float32:
		return int(math.Round(float64(x)))
	case int:
		return x
	case int64:
		return int(x)
	case json.Number:
		if i, err := x.Int64(); err == nil {
			return int(i)
		}
	}
	return 0
}

func toInt64(v any) int64 {
	switch x := v.(type) {
	case float64:
		return int64(math.Round(x))
	case float32:
		return int64(math.Round(float64(x)))
	case int:
		return int64(x)
	case int64:
		return x
	case json.Number:
		if i, err := x.Int64(); err == nil {
			return i
		}
	}
	return 0
}

func toFloat(v any) float64 {
	switch x := v.(type) {
	case float64:
		return x
	case float32:
		return float64(x)
	case int:
		return float64(x)
	case int64:
		return float64(x)
	case json.Number:
		if f, err := x.Float64(); err == nil {
			return f
		}
	}
	return 0
}

func toBool(v any) bool {
	if b, ok := v.(bool); ok {
		return b
	}
	return false
}

func round10(v float64) int {
	return int(math.Round(v/10.0) * 10)
}

// roundCD округляет cooldown до int (секунды)
func roundCD(v float64) int { return int(math.Round(v)) }

func clamp01pct(v int) int {
	if v < 0 {
		return 0
	}
	if v > 100 {
		return 100
	}
	return v
}

func firstNonZero(vals ...int) int {
	for _, v := range vals {
		if v != 0 {
			return v
		}
	}
	return 0
}

func itoa(i int) string { // маленький локальный helper, чтобы не тянуть strconv
	// быстро и без аллокаций ради простоты — диапазон 0..9
	return string('0' + rune(i))
}
