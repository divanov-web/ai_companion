##Предварительные варианты

Требования
Должно работать под Win 11, можно псевдореалтайм. 
Желательно, чтобы это запускалось в самом Go или могло быть вызвано из приложения. 
Желательно, чтобы оно умело понимать конец фразы, как в программах-чатах, достаточно получать final result вместо particular result

### через whisper.cpp
Microphone (WASAPI shared)
↓
FFmpeg (chunking → 16kHz mono PCM WAV)
↓
whisper-cli (whisper.cpp)
↓
stdout → Aggregator (PowerShell / Go)

### через Yandex SpeechKit STT (стриминг)
аудиозахват + gRPC/stream + обработка partial/final

