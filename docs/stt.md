##Предварительные варианты
### через whisper.cpp
Microphone (WASAPI shared)
↓
FFmpeg (chunking → 16kHz mono PCM WAV)
↓
whisper-cli (whisper.cpp)
↓
stdout → Aggregator (PowerShell / Go)

