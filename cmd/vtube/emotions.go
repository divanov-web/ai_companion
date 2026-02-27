package main

import (
	"OpenAIClient/internal/config"
	"OpenAIClient/internal/service/vtube"
	"context"
	"errors"
	"math/rand"
	"slices"
	"strings"
	"time"

	"go.uber.org/zap"
)

// runEmotionsTest запускает on-demand тест эмоций через внутренний клиент VTube Studio.
// Алгоритм:
// 1) Загружаем конфиг и логгер.
// 2) Стартуем VTS-клиент (однократно грузит хоткеи и закрывает соединение).
// 3) Собираем уникальные теги из cfg.CharacterList[].Tags.
// 4) Случайно выбираем 1..3 тегов, логируем и отправляем TriggerByNames.
// 5) Ждём короткую паузу и отправляем TriggerReset.
func runEmotionsTest(ctx context.Context) error {
	cfg := config.NewConfig()
	if !cfg.VTube.Enabled {
		return errors.New("VTube в конфиге отключён (VTUBE_ENABLED=false)")
	}
	if strings.TrimSpace(cfg.VTubeAPIKey) == "" {
		return errors.New("VTUBE_API_KEY не задан — укажите токен аутентификации VTube Studio в окружении/.env")
	}

	// Логгер
	zl, _ := zap.NewDevelopment()
	log := zl.Sugar()
	defer zl.Sync()

	// Клиент VTS и первичная загрузка хоткеев
	client := vtube.New(cfg.VTube, cfg.VTubeAPIKey, log)
	cctx, cancel := context.WithTimeoutCause(ctx, 10*time.Second, errors.New("vtube emotions test: start timeout"))
	defer cancel()
	if err := client.Start(cctx); err != nil {
		log.Errorw("VTS start failed", "error", err)
		return err
	}

	// Собираем все уникальные теги из CharacterList
	uniq := make(map[string]struct{})
	for _, ch := range cfg.CharacterList {
		for _, t := range ch.Tags {
			t = strings.TrimSpace(t)
			if t == "" {
				continue
			}
			uniq[t] = struct{}{}
		}
	}
	all := make([]string, 0, len(uniq))
	for k := range uniq {
		all = append(all, k)
	}
	slices.Sort(all)
	if len(all) == 0 {
		log.Warnw("В конфиге нет тегов эмоций (CharacterList[].Tags пусты) — нечего триггерить")
		return nil
	}

	// Выбираем случайные 1..3 тега
	rnd := rand.New(rand.NewSource(time.Now().UnixNano()))
	count := 1 + rnd.Intn(min(3, len(all))) // хотя бы 1, максимум 3 или len(all)
	// Перемешаем копию и возьмём первые count
	shuffled := append([]string(nil), all...)
	rnd.Shuffle(len(shuffled), func(i, j int) { shuffled[i], shuffled[j] = shuffled[j], shuffled[i] })
	pick := shuffled[:count]

	log.Infow("Emotions test: picked tags", "tags", pick)
	if err := client.TriggerByNames(pick); err != nil {
		log.Warnw("VTS trigger failed", "error", err)
		return err
	}

	// Подержим эмоцию и сбросим
	time.Sleep(5 * time.Second)
	if err := client.TriggerReset(); err != nil {
		log.Warnw("VTS reset failed", "error", err)
		return err
	}

	log.Infow("Emotions test finished")
	return nil
}
