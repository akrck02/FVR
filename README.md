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

## Qué hay dónde

| Paquete | Qué hace |
|---|---|
| `headset` | `headset.Run(app.Options, step)`: el sustituto de `app.Run` de go-meta-quest. Ventana, cabeza, manos, ojos, paneles y capturas. |
| `desktopgl` | `gles.Functions` sobre el OpenGL 2.1 del ordenador, con los shaders GLSL ES pasados a GLSL 1.20. |
| `cmd/playground` | El primer programa: el tablero de `cmd/panel` de go-meta-quest, en la ventana. |

Un mismo programa elige casco o ventana con una etiqueta de compilación: `app.Run` en un fichero
`//go:build android` y `headset.Run` en otro `//go:build !android`.

Más detalle en [docs/](docs/): [cómo funciona](docs/1-como-funciona.md) y
[las fases siguientes](docs/2-fases.md) (mandos con giroscopio y webcam).

## Requisitos

Go 1.25 y, por ahora, go-renderer y go-meta-quest clonados al lado (`../go-renderer`,
`../go-meta-quest`): `go.mod` los sustituye por esas carpetas mientras se trabaja en las tres a la vez.

## Divulgación de IA

Escribo casi todo mi código a mano, así que quiero dejarlo claro: **este proyecto lo ha escrito un
asistente de IA (Claude, de Anthropic) bajo mi dirección.** Cada commit hecho así lleva una línea
`Co-Authored-By: Claude`.

**Estado: pendiente de revisión a fondo.** Compila y sus pruebas pasan, pero aún no lo he revisado
línea a línea. Tómalo como un borrador que funciona, no como código que avalo.
