# Cómo funciona

`headset.Run` hace en una ventana lo que `app.Run` de go-meta-quest hace en las gafas: cada
fotograma le entrega al programa un `app.Frame` y el programa dibuja los ojos y los paneles.

## Cada fotograma

1. **Cabeza.** Está de pie en el centro del espacio del jugador, a 1,6 m del suelo (`Head`). El
   ratón con el botón derecho, las flechas o el stick izquierdo de un mando la giran (guiñada y cabeceo, sin pasar de 85° arriba
   o abajo). `HeadTracked` es siempre verdadero.
2. **Ojos.** Dos, separados 63 mm a lo ancho de la cabeza (`Eyes`). Ven ±45° en vertical y lo que
   pida la forma de la imagen en horizontal (`FieldFor`). Cada uno dibuja en su propio framebuffer
   con profundidad, del tamaño de la ventana (o de media ventana con los dos ojos).
3. **Manos.** La derecha está en reposo junto a la cabeza (`selftest.HandBeside`) y apunta a lo que
   hay bajo el cursor: el panel más cercano que corte la mirada por ese punto, o 10 m más allá
   (`RightHand`). Así el rayo sale de la mano y cae bajo el cursor. La izquierda descansa al otro
   lado mirando al frente, o gira la palma hacia la cara con Tab (`LeftHand`).
4. **Mandos con giroscopio**, si hay (ver abajo): cada uno pasa a ser su mano, y sus sticks y
   botones se suman a los de las teclas (`WithPads`). Su stick izquierdo no: gira la cabeza, como
   las flechas, porque en la ventana no hay casco que mueva la vista (`LookWithPads`).
5. **Mandos.** Las teclas se convierten en un `openxr.Controls` (`Keys.Controls`) y este se aplica
   al `input.State` como en las gafas (`Controls.Apply`): sticks, A/B/X/Y, clic de los sticks y
   menú. La rueda empuja el stick derecho el tiempo justo para desplazar una muesca (`Wheel`).
6. **El programa** recibe el fotograma y dibuja.
7. **Composición.** Cada panel visible se dibuja sobre cada ojo como un rectángulo en su pose, con
   el alfa premultiplicado, por encima de la escena (como el compositor del casco). Luego los ojos
   se copian a la ventana.

## El mando con giroscopio

`gamepad` carga SDL3 del sistema al arrancar (con purego: se compila sin él) y abre hasta dos mandos
con giroscopio y acelerómetro, uno por mano. Un Joy-Con (L) va a la izquierda y un (R) a la derecha;
un mando entero, a la derecha, o a la izquierda si la derecha ya está cogida. A SDL se le pide que no
junte los Joy-Con en un solo mando y que los lea de pie (su modo vertical): así cada uno se sujeta
como un mando del casco y tiene su mitad de los botones. Cada muestra de los sensores llega como
evento (unas 200 por segundo), con su propia marca de tiempo; los Joy-Con tardan un par de segundos
en empezar a mandarlas.

`motion.Orientation` las junta en una orientación:

- El giroscopio la gira.
- La gravedad, que el acelerómetro lee hacia arriba en reposo, corrige poco a poco la inclinación
  (filtro de Mahony). Solo se fía de la lectura cuando está cerca de 1 g: un movimiento brusco
  añade su propia aceleración.
- Nada corrige la guiñada, porque el mando no tiene brújula. Por eso un botón de cada mando la
  centra hacia donde mira la cabeza: Captura en el Joy-Con (L), Home en el (R), − en un mando entero.
- Mientras el mando está quieto medio segundo, aprende el sesgo del giroscopio (lo que marca en
  reposo) y lo descuenta, para que la guiñada apenas se desvíe.

`motion.Hand` coloca la mano: el codo a un lado del cuerpo, bajo la cabeza y girando con ella, y el
antebrazo de 30 cm girando con el mando.

## OpenGL

Los renderizadores de go-renderer (`gles`) hablan con una interfaz, `gles.Functions`, con la forma
de OpenGL ES 2.0. `desktopgl` la implementa sobre el contexto OpenGL 2.1 que da cualquier escritorio
(el de macOS por defecto). La única diferencia es el lenguaje de shaders: `#version 100` pasa a
`#version 120` y se quitan las líneas `precision …;` (`desktopgl.Shader`).

Dos detalles del compositor, porque los renderizadores cuentan con el estado que dejan:

- Lee los paneles desde la unidad de textura 7, que ellos no usan.
- Los renderizadores activan sus atributos de vértice una vez y esperan que sigan activos. El
  compositor los apaga para dibujar los paneles y los deja como estaban al terminar.

## Un programa del casco, en la ventana

go-meta-quest, fuera del casco, no abre ninguna sesión: su `app.Run` le pasa el programa a quien se
haya registrado con `app.UseStandIn`, y falla con `app.ErrNoHeadset` si nadie lo ha hecho. El
paquete `headset` se registra al cargarse, así que basta con que el programa lo importe.

`cmd/fvr` consigue eso sin tocar el programa. Lee los ficheros de su carpeta y `attach.Changes`
decide, con las mismas reglas que `go build` (sufijo del nombre y línea `//go:build`), qué ve el
compilador:

- lo que solo se compila para el casco se enseña sin la restricción (si la restricción es el nombre,
  con otro: `algo_android.go` pasa a `algo_android_fvr.go`);
- lo que solo se compila fuera del casco se esconde;
- se añade `fvr_attach.go`, con `import _ "github.com/akrck02/FVR/headset"`.

Todo va en un fichero de *overlay* de `go build`, con un `go.work` temporal que junta FVR, los
módulos que usa y el del programa, para que todos compilen contra las mismas carpetas. Si el programa
se compila para el casco con etiquetas propias (`-tags`), las mismas cuentan para decidir qué fichero
es de quién. El ejecutable corre en la carpeta temporal, como en el casco lejos de su código: lo que
escriba donde corre (un socket, un fichero) no acaba en el repositorio del programa. Las opciones de la
ventana (`-both`, `-capture`, `-frame`) llegan por el entorno (`FVR_BOTH`, `FVR_CAPTURE`,
`FVR_FRAME`, leídas por `headset.WindowFromEnvironment`).

Un programa cuyo `main` de escritorio no está apartado del casco por una etiqueta no encaja: fvr no
puede saber cuál de los dos es el del casco. Con una etiqueta propia para el casco sí:
`-tags casco` aparta el `main` de escritorio (`//go:build !casco`) y deja el del casco.

## Lo que no hay (aún)

- `ShowsRoom` es siempre falso: no hay passthrough (fase 3, con la webcam).
- `SRGB` es falso: la ventana no codifica sRGB, así que los renderizadores dan el color ya corregido.
- Solo vibra una mano con mando: las vibraciones se ven también en el título.
