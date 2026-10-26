# FVR

Un *playground* para probar cosas de realidad virtual sin tener siempre las gafas a mano. Un
programa escrito para unas Meta Quest con [go-meta-quest](https://github.com/akrck02/go-meta-quest)
se ejecuta, tal cual, en una ventana del ordenador: el ratón y el teclado hacen de cabeza y de
mandos, la ventana muestra los ojos, y los paneles se colocan sobre ellos como lo haría el casco.

Usa [go-renderer](https://github.com/akrck02/go-renderer) (sus renderizadores OpenGL ES) y
go-meta-quest (el `app.Frame` que reciben los programas, y sus piezas de apuntar, colocar y
explorar).

```bash
go run ./cmd/playground          # la isla de ejemplo con un tablero delante
go run ./cmd/playground -both    # los dos ojos, uno al lado del otro
go run ./cmd/playground -capture captura.png   # guarda un fotograma y termina
```

## Juegos del casco, tal cual

`cmd/fvr` ejecuta en la ventana cualquier programa escrito para el casco con `app.Run` o
`explore.Run`, sin tocar un solo fichero suyo:

```bash
go run ./cmd/fvr ../go-meta-quest/cmd/explorer               # la isla, para andar por ella
go run ./cmd/fvr -both ../go-meta-quest/cmd/panel            # los dos ojos
go run ./cmd/fvr -capture isla.png -frame 60 ../go-meta-quest/cmd/explorer
go run ./cmd/fvr ../mi-juego -- -nivel 2                     # lo que va tras -- es del juego
go run ./cmd/fvr -tags demo,quest ../otro-juego              # las etiquetas con que se compila para el casco
```

Compila el programa con un `go.work` temporal (FVR, go-renderer, go-meta-quest y el módulo del
programa) y `go build -overlay`, y lo ejecuta en esa carpeta temporal, lejos de su código como en el
casco: los ficheros que solo son del casco (`//go:build android`,
`_android.go`) se le enseñan al compilador sin esa restricción, los que solo son del ordenador (el
típico `elsewhere.go` que dice «compílame para el casco») se esconden, y se añade uno que importa
`headset`. Ese paquete se registra con `app.UseStandIn`, así que el `app.Run` del programa abre la
ventana de FVR en vez de las gafas. Funcionan `explorer`, `panel`, `cube`, `room` (sin passthrough:
el cubo sobre el cielo) y `clear`.

## Controles

| Tecla / ratón | En el casco |
|---|---|
| Ratón | Mando derecho: apunta a lo que hay bajo el cursor |
| Clic izquierdo | Gatillo derecho |
| Clic derecho mantenido, o flechas | Girar la cabeza |
| W A S D | Stick izquierdo |
| Q E · R F · rueda | Stick derecho (lados · arriba y abajo) |
| G | Agarre derecho |
| Espacio · B · X · Y | Botones A · B · X · Y |
| L · K | Pulsar el stick izquierdo · derecho |
| Tab | Palma izquierda hacia la cara |
| M | Menú |
| V | Un ojo o los dos |
| Esc | Salir |

Las vibraciones se ven en el título de la ventana.

### Con mandos con giroscopio

Los mandos con giroscopio emparejados por Bluetooth hacen de manos: cada uno apunta donde apunta él.
Hace falta SDL3 (`brew install sdl3`); la mano que no tenga mando sigue con el teclado y el ratón.

- **Dos Joy-Con**: el (L) es la mano izquierda y el (R) la derecha. Se sujetan de pie, como los
  mandos del casco, apuntando con el extremo del stick.
- **Un mando entero** (Pro Controller, 8BitDo en **modo Switch**): la mano derecha, o la izquierda si
  ya hay un Joy-Con (R).

| Mando | En el casco |
|---|---|
| Apuntar con el mando | Su mano |
| Captura (Joy-Con L) · Home (Joy-Con R) · − (mando entero) | Centrarlo: apúntalo al frente y púlsalo |
| ZR · R | Gatillo · agarre derechos |
| ZL · L | Gatillo · agarre izquierdos |
| Stick izquierdo | Girar la cabeza (la cámara); para andar, W A S D |
| Stick derecho | Stick derecho |
| Botón de abajo · derecha | A · B |
| Cruceta abajo · arriba (Joy-Con L), botón izquierda · arriba (mando entero) | X · Y |
| − (Joy-Con L) · + | Menú |

`go run ./cmd/pad` enseña lo que dan los mandos (hacia dónde apunta cada uno, sticks, botones) sin
abrir ventana.

## Qué hay dónde

| Paquete | Qué hace |
|---|---|
| `headset` | `headset.Run(app.Options, step)`: el sustituto de `app.Run` de go-meta-quest. Ventana, cabeza, manos, ojos, paneles y capturas. |
| `gamepad` | Lee hasta dos mandos con giroscopio por SDL3 (cargado con purego, sin cgo), uno por mano: sticks, botones, vibración y hacia dónde apunta cada uno. |
| `motion` | Las cuentas del mando: un filtro de Mahony que junta giroscopio y acelerómetro, el sesgo del giroscopio aprendido en reposo, el centrado y un modelo de brazo. |
| `desktopgl` | `gles.Functions` sobre el OpenGL 2.1 del ordenador, con los shaders GLSL ES pasados a GLSL 1.20. |
| `cmd/playground` | El primer programa: el tablero de `cmd/panel` de go-meta-quest, en la ventana. |
| `cmd/pad` | Lo que dan los mandos, en la terminal. |
| `attach` | Qué enseñar al compilador para que un programa del casco se compile en el ordenador: qué ficheros mostrar sin su restricción, cuáles esconder y el que se añade. |
| `cmd/fvr` | Ejecuta un programa del casco en la ventana (con `attach` y `go build -overlay`). |

Un programa del casco no necesita nada para correr aquí: `cmd/fvr` lo lanza tal cual. Uno que quiera
la ventana por su cuenta importa `headset` (o llama a `headset.Run` directamente, como `cmd/playground`).

Más detalle en [docs/](docs/): [cómo funciona](docs/1-como-funciona.md) y
[las fases siguientes](docs/2-fases.md) (webcam).

## Requisitos

Go 1.25 y, por ahora, go-renderer y go-meta-quest clonados al lado (`../go-renderer`,
`../go-meta-quest`): `go.mod` los sustituye por esas carpetas mientras se trabaja en las tres a la vez.

## Divulgación de IA

Escribo casi todo mi código a mano, así que quiero dejarlo claro: **este proyecto lo ha escrito un
asistente de IA (Claude, de Anthropic) bajo mi dirección.** Cada commit hecho así lleva una línea
`Co-Authored-By: Claude`.

**Estado: pendiente de revisión a fondo.** Compila y sus pruebas pasan, pero aún no lo he revisado
línea a línea. Tómalo como un borrador que funciona, no como código que avalo.
