# Fases

## 1. Ventana, ratón y teclado ✅

`headset.Run`, el adaptador `desktopgl` y `cmd/playground`. Un programa de go-meta-quest corre en
el ordenador sin cambios.

## 2. Mandos con giroscopio como manos

Joy-Con, Pro Controller de Switch o un 8BitDo Ultimate (este solo da giroscopio en modo Switch por
Bluetooth, donde se presenta como Pro Controller).

- Leerlos con **SDL3** (`brew install sdl3`; la 3.4 trae el arreglo del acelerómetro de los Joy-Con
  por Bluetooth en macOS) desde Go mediante enlaces `purego`, sin cgo.
- Fusionar giroscopio y acelerómetro (filtro de Madgwick o Mahony) en una orientación por mando,
  con un botón para recentrar.
- Un modelo de brazo (como el de Daydream) da la posición de la mano a partir de la orientación y
  la cabeza.
- Sticks, gatillos y botones al `openxr.Hand`; la vibración a los motores HD.

## 3. La webcam como passthrough

- Capturar con la API de cámara de SDL3 (o una tubería de `ffmpeg` con avfoundation).
- Mostrarla detrás de la escena cuando el programa pide `Options.Passthrough` (`ShowsRoom`),
  primero como fondo fijo y después proyectada en la habitación.
