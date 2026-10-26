# Cómo funciona

`headset.Run` hace en una ventana lo que `app.Run` de go-meta-quest hace en las gafas: cada
fotograma le entrega al programa un `app.Frame` y el programa dibuja los ojos y los paneles.

## Cada fotograma

1. **Cabeza.** Está de pie en el centro del espacio del jugador, a 1,6 m del suelo (`Head`). El
   ratón con el botón derecho, o las flechas, la giran (guiñada y cabeceo, sin pasar de 85° arriba
   o abajo). `HeadTracked` es siempre verdadero.
2. **Ojos.** Dos, separados 63 mm a lo ancho de la cabeza (`Eyes`). Ven ±45° en vertical y lo que
   pida la forma de la imagen en horizontal (`FieldFor`). Cada uno dibuja en su propio framebuffer
   con profundidad, del tamaño de la ventana (o de media ventana con los dos ojos).
3. **Manos.** La derecha está en reposo junto a la cabeza (`selftest.HandBeside`) y apunta a lo que
   hay bajo el cursor: el panel más cercano que corte la mirada por ese punto, o 10 m más allá
   (`RightHand`). Así el rayo sale de la mano y cae bajo el cursor. La izquierda descansa al otro
   lado mirando al frente, o gira la palma hacia la cara con Tab (`LeftHand`).
4. **Mandos.** Las teclas se convierten en un `openxr.Controls` (`Keys.Controls`) y este se aplica
   al `input.State` como en las gafas (`Controls.Apply`): sticks, A/B/X/Y, clic de los sticks y
   menú. La rueda empuja el stick derecho el tiempo justo para desplazar una muesca (`Wheel`).
5. **El programa** recibe el fotograma y dibuja.
6. **Composición.** Cada panel visible se dibuja sobre cada ojo como un rectángulo en su pose, con
   el alfa premultiplicado, por encima de la escena (como el compositor del casco). Luego los ojos
   se copian a la ventana.

## OpenGL

Los renderizadores de go-renderer (`gles`) hablan con una interfaz, `gles.Functions`, con la forma
de OpenGL ES 2.0. `desktopgl` la implementa sobre el contexto OpenGL 2.1 que da cualquier escritorio
(el de macOS por defecto). La única diferencia es el lenguaje de shaders: `#version 100` pasa a
`#version 120` y se quitan las líneas `precision …;` (`desktopgl.Shader`).

Dos detalles del compositor, porque los renderizadores cuentan con el estado que dejan:

- Lee los paneles desde la unidad de textura 7, que ellos no usan.
- Los renderizadores activan sus atributos de vértice una vez y esperan que sigan activos. El
  compositor los apaga para dibujar los paneles y los deja como estaban al terminar.

## Lo que no hay (aún)

- `ShowsRoom` es siempre falso: no hay passthrough (fase 3, con la webcam).
- `SRGB` es falso: la ventana no codifica sRGB, así que los renderizadores dan el color ya corregido.
- Nada vibra: las vibraciones se ven en el título de la ventana.
