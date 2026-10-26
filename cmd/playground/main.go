// The playground's first program: go-meta-quest's panel demo, run in a window instead of the
// headset. A board floats in front of you over go-renderer's sample island; either hand points at
// it, a beam showing where, and its trigger lights a tile or puts it out. The mouse is the right
// controller (it points where the cursor is, and its left button is the trigger) unless pads with
// motion sensors are the hands, the keys are the sticks and the buttons, and the right button held
// turns the head.
//
//	go run ./cmd/playground
//	go run ./cmd/playground -capture board.png
package main

import (
	"flag"
	"fmt"
	"log"
	"os"
	"time"

	"github.com/akrck02/FVR/headset"
	"github.com/akrck02/go-meta-quest/app"
	"github.com/akrck02/go-meta-quest/explore"
	"github.com/akrck02/go-meta-quest/pointer"
	"github.com/akrck02/go-renderer/camera"
	"github.com/akrck02/go-renderer/gles"
	"github.com/akrck02/go-renderer/graphics"
	"github.com/akrck02/go-renderer/input"
	"github.com/akrck02/go-renderer/opengl/overlay"
	"github.com/akrck02/go-renderer/scene"
	"github.com/akrck02/go-renderer/scene/sample"
)

// The keys, for whoever starts the program.
const help = `FVR · playground
  ratón              mando derecho: apunta donde está el cursor; clic izquierdo = gatillo
  clic derecho       mantener y arrastrar para girar la cabeza (también las flechas)
  W A S D            stick izquierdo (andar)
  Q E / R F          stick derecho (girar / arriba y abajo); la rueda también
  G                  agarre derecho
  espacio B X Y      botones A, B, X, Y
  L K                pulsar los sticks
  Tab                palma izquierda hacia la cara
  M                  menú
  V                  un ojo o los dos
  Esc                salir

  Con mandos con giroscopio por Bluetooth, cada mando es su mano y apunta donde apunta él: dos
  Joy-Con son las dos manos, un Pro Controller o un 8BitDo en modo Switch la derecha. Captura
  (Joy-Con L), Home (Joy-Con R) o − (mando entero) lo centra hacia donde miras; ZR/ZL gatillos,
  R/L agarres, el stick izquierdo gira la cabeza (la cámara), y el derecho y los botones como los
  del casco.`

// boardPose is where the board stands in the room: a step and a half ahead, its middle a little
// under a standing person's eyes, facing them.
var boardPose = graphics.Pose{Orientation: graphics.IdentityQuaternion(), Position: graphics.Vector{0, 1.3, -1.5, 1}}

// demo is the program's state between frames.
type demo struct {
	explorer *explore.Explorer
	board    *board
	pointer  pointer.Pointer
	beams    [2]*pointer.Beam // a beam on each hand, by app.LeftHand and app.RightHand
	pointing int              // the hand the board follows
	panel    *input.State     // the board's interface's own input: its cursor and button
	scene    *gles.SceneRenderer
	flat     *gles.FlatRenderer
	stale    bool // the board's picture changed since it was last drawn
}

func main() {
	window := headset.Window{Title: "FVR · playground"}
	flag.BoolVar(&window.SideBySide, "both", false, "show both eyes side by side")
	flag.StringVar(&window.Capture, "capture", "", "save the window as a PNG at this path, then end")
	flag.IntVar(&window.CaptureFrame, "frame", 30, "the frame -capture saves")
	flag.Parse()
	fmt.Fprintln(os.Stderr, help)

	world := sample.Island()
	made := &demo{board: newBoard(), stale: true, panel: input.NewState(), pointing: app.RightHand}
	made.beams[app.LeftHand] = pointer.NewBeam(graphics.Vector{1, 0.7, 0.4, 1})
	made.beams[app.RightHand] = pointer.NewBeam(graphics.Vector{0.4, 0.8, 1, 1})
	made.explorer = explore.New(world, explore.Options{Start: startRig(world), WalkMetresSecond: 3, TurnRadiansSecond: 2})
	world.Nodes = append(world.Nodes, made.beams[app.LeftHand].Node, made.beams[app.RightHand].Node)
	options := app.Options{ApplicationName: "Playground", Panels: []app.PanelSize{{Width: boardWidth, Height: boardHeight}}}
	if err := headset.RunIn(window, options, made.frame); err != nil {
		log.Fatal(err)
	}
}

// startRig stands the room on the island's ground at its middle.
func startRig(world *scene.Scene) camera.Rig {
	rig := camera.Rig{Position: graphics.Vector{0, 0, 0, 1}, Scale: explore.PersonScale}
	if ground, found := world.GroundHeight(0, 0); found {
		rig.Position[1] = ground
	}
	return rig
}

func (made *demo) frame(frame app.Frame) bool {
	panel := frame.Panels[0]
	panel.Pose = boardPose
	panel.FitWidth(boardMetres)
	panel.Shown = true
	made.explorer.Steer(frame.Input, frame.Seconds, frame.Head, frame.HeadTracked)
	made.point(frame)
	made.explorer.Follow([2]graphics.Pose{frame.Hands[app.LeftHand].Grip, frame.Hands[app.RightHand].Grip},
		[2]bool{frame.Hands[app.LeftHand].Tracked, frame.Hands[app.RightHand].Tracked})
	if !frame.ShouldRender {
		return true
	}
	if !made.prepare(frame) {
		return false
	}
	made.drawBoard(frame)
	made.scene.LinearOutput = frame.SRGB
	views, covering, aspect := made.explorer.Views(frame.Eyes)
	made.scene.DrawViews(made.explorer.World(), covering, aspect, views)
	return true
}

// point aims the hands at the board: each beam reaches it, and the board follows one hand (the one
// on it, the one already there when both are): the tile under it lights its border, and a pull of
// its trigger lights the tile with a buzz in that hand.
func (made *demo) point(frame app.Frame) {
	board := frame.Panels[0]
	var hits [2]pointer.Hit
	var on [2]bool
	for index, hand := range frame.Hands {
		if hand.Tracked {
			hits[index], on[index] = pointer.HitPanel(pointer.FromAim(hand.Aim), board)
		}
		made.beams[index].Place(made.explorer.Rig(), hand, hits[index].Distance)
	}
	if !on[made.pointing] && on[1-made.pointing] {
		made.pointing = 1 - made.pointing
	}
	hand := made.pointing
	hit, pointing := made.pointer.Update(made.panel, frame.Hands[hand], board, frame.Seconds)
	if made.board.hover(hit.X, hit.Y, pointing) {
		made.stale = true
		if made.board.hovered >= 0 {
			frame.Vibrate(hand, 0.15, 10*time.Millisecond)
		}
	}
	if made.panel.ButtonPressed(input.MouseLeft) && made.board.click() {
		made.stale = true
		frame.Vibrate(hand, 0.6, 40*time.Millisecond)
	}
	made.panel.Feed().EndFrame()
}

// prepare makes the renderers the first time a frame is drawn, where OpenGL is current.
func (made *demo) prepare(frame app.Frame) bool {
	if made.scene != nil {
		return true
	}
	scene, err := gles.NewSceneRenderer(frame.Functions, overlay.Fonts{})
	if err != nil {
		log.Print(err)
		return false
	}
	flat, err := gles.NewFlatRenderer(frame.Functions, overlay.Fonts{})
	if err != nil {
		log.Print(err)
		return false
	}
	made.scene, made.flat = scene, flat
	return true
}

// drawBoard draws the board's picture again when it changed; the panel keeps showing the last.
func (made *demo) drawBoard(frame app.Frame) {
	if !made.stale {
		return
	}
	target, err := frame.Panels[0].Target()
	if err != nil {
		log.Print(err)
		return
	}
	made.flat.Target, made.flat.LinearOutput = target, frame.SRGB
	made.flat.Draw(made.board.draw(), boardColour, boardWidth, boardHeight, 1)
	made.stale = false
}
