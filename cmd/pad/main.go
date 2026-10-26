// Shows what the game pads with motion sensors give, a hand to a line, a few times a second: where
// each points (yaw and pitch, in degrees), its sticks, triggers and buttons. A pad's recenter
// button (Capture on a left Joy-Con, Home on a right one, − on a whole pad) points it ahead again.
// To see that the pads are read, and how they move, before a program uses them.
//
//	go run ./cmd/pad
//	go run ./cmd/pad -seconds 10
package main

import (
	"flag"
	"fmt"
	"log"
	"math"
	"strings"
	"time"

	"github.com/akrck02/FVR/gamepad"
	"github.com/akrck02/go-renderer/graphics"
)

var buttonNames = [...]string{"S", "E", "W", "N", "−", "home", "+", "LS", "RS", "L", "R", "↑", "↓", "←", "→", "captura"}

var handNames = [2]string{gamepad.LeftHand: "izquierda", gamepad.RightHand: "derecha  "}

func main() {
	seconds := flag.Float64("seconds", 0, "stop after this long (0: never)")
	flag.Parse()
	reader, err := gamepad.Open()
	if err != nil {
		log.Fatal(err)
	}
	defer reader.Close()
	start, shown, were := time.Now(), time.Time{}, [2]gamepad.State{}
	for *seconds == 0 || time.Since(start).Seconds() < *seconds {
		states := reader.Read()
		for hand, state := range states {
			if state.Pressed(state.RecenterButton(), were[hand]) {
				reader.Recenter(hand, 0)
			}
		}
		were = states
		if time.Since(shown) > 250*time.Millisecond {
			shown = time.Now()
			for hand, state := range states {
				fmt.Printf("mano %s · %s\n", handNames[hand], describe(state))
			}
		}
		time.Sleep(4 * time.Millisecond)
	}
}

func describe(state gamepad.State) string {
	if !state.Connected {
		return "sin mando"
	}
	ahead := state.Rotation.Rotate(graphics.Vector{0, 0, -1, 0})
	yaw := math.Atan2(float64(-ahead[0]), float64(-ahead[2])) * 180 / math.Pi
	pitch := math.Asin(math.Max(-1, math.Min(1, float64(ahead[1])))) * 180 / math.Pi
	var held []string
	for index, down := range state.Buttons {
		if down {
			held = append(held, buttonNames[index])
		}
	}
	axes := state.Axes
	return fmt.Sprintf("%s · guiñada %6.1f° cabeceo %6.1f° · izq %+.2f %+.2f der %+.2f %+.2f · gatillos %.2f %.2f · %s",
		state.Name, yaw, pitch, axes[gamepad.LeftX], axes[gamepad.LeftY], axes[gamepad.RightX], axes[gamepad.RightY],
		axes[gamepad.LeftTrigger], axes[gamepad.RightTrigger], strings.Join(held, " "))
}
