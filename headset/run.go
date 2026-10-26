// Package headset stands in for a Meta Quest on a computer: it runs a program written for
// go-meta-quest's app.Run in a window, handing it the same app.Frame every frame. The mouse and the
// keyboard are the head and the controllers, the window shows the eyes, and the panels the program
// asks for are laid over them as the headset would.
//
// A program picks the real headset or this one with a build tag: app.Run in a file for android,
// headset.Run in one for the rest.
package headset

import (
	"fmt"
	"log"
	"math"
	"runtime"
	"time"

	"github.com/akrck02/FVR/desktopgl"
	"github.com/akrck02/FVR/gamepad"
	"github.com/akrck02/go-meta-quest/app"
	"github.com/akrck02/go-meta-quest/place"
	"github.com/akrck02/go-renderer/gles"
	"github.com/akrck02/go-renderer/graphics"
	"github.com/akrck02/go-renderer/input"
	"github.com/akrck02/go-renderer/openxr"
	"github.com/go-gl/glfw/v3.4/glfw"
)

// OpenGL and the window must stay on the thread that made them: the main one, on macOS.
func init() { runtime.LockOSThread() }

// How the mouse and the arrows turn the head.
const (
	radiansPerPixel   = 0.004
	radiansPerSecond  = 1.6
	longestFrame      = 0.1 // seconds: what Frame.Seconds is kept within, as on the headset
	vibrationTitleFor = 150 * time.Millisecond
)

// Window is how the window is opened, and what it does besides showing the eyes.
type Window struct {
	// Title names the window; the program's name (app.Options.ApplicationName) when empty.
	Title string
	// Width and Height are the window's size, in the screen's points; 1280 × 720 when 0.
	Width, Height int
	// SideBySide shows both eyes, left and right, each in half the window; else the left eye
	// fills it. V switches between them.
	SideBySide bool
	// Capture, when set, saves the window as a PNG at that path after CaptureFrame frames
	// (1 when 0) and ends the program: to see what a program draws without looking.
	Capture      string
	CaptureFrame int
}

// Run runs a program in a window of 1280 × 720 points until it returns false or the window is
// closed. It must be called from main.
func Run(options app.Options, step func(app.Frame) bool) error {
	return RunIn(Window{}, options, step)
}

// RunIn runs a program in the window asked for, as Run.
func RunIn(window Window, options app.Options, step func(app.Frame) bool) error {
	if err := app.CheckPanels(options.Panels); err != nil {
		return err
	}
	window = withDefaults(window, options)
	if err := glfw.Init(); err != nil {
		return fmt.Errorf("headset: no window system: %w", err)
	}
	defer glfw.Terminate()
	glfw.WindowHint(glfw.ContextVersionMajor, 2)
	glfw.WindowHint(glfw.ContextVersionMinor, 1)
	shown, err := glfw.CreateWindow(window.Width, window.Height, window.Title, nil, nil)
	if err != nil {
		return fmt.Errorf("headset: no window: %w", err)
	}
	defer shown.Destroy()
	shown.MakeContextCurrent()
	glfw.SwapInterval(1)
	if err := desktopgl.Init(); err != nil {
		return fmt.Errorf("headset: no OpenGL: %w", err)
	}
	loop, err := newLoop(shown, window, options)
	if err != nil {
		return err
	}
	defer loop.delete()
	if options.Shown != nil {
		options.Shown()
	}
	if err := loop.run(step); err != nil {
		return err
	}
	if options.Hidden != nil {
		options.Hidden()
	}
	return nil
}

func withDefaults(window Window, options app.Options) Window {
	if window.Title == "" {
		window.Title = options.ApplicationName
	}
	if window.Title == "" {
		window.Title = "FVR"
	}
	if window.Width <= 0 || window.Height <= 0 {
		window.Width, window.Height = 1280, 720
	}
	if window.CaptureFrame <= 0 {
		window.CaptureFrame = 1
	}
	return window
}

// loop is the window between frames.
type loop struct {
	window   *glfw.Window
	settings Window
	options  app.Options
	head     Head
	wheel    Wheel
	state    *input.State
	eyes     [2]*target
	panels   []*openxr.Panel
	pictures []*target
	composer *composer
	haptics  *vibrations
	pads     *gamepad.Reader  // nil without SDL3
	padsNow  [2]gamepad.State // the pads as last read, by hand
	padsWere [2]gamepad.State // and the read before

	sideBySide       bool
	looking          bool    // the right button turns the head
	lookX, lookY     float64 // where the cursor was, while looking
	menuWas, viewWas bool    // M and V, last frame: they act as they go down
	frames           int
}

func newLoop(window *glfw.Window, settings Window, options app.Options) (*loop, error) {
	made := &loop{window: window, settings: settings, options: options, head: StandingHead(),
		state: input.NewState(), sideBySide: settings.SideBySide, haptics: &vibrations{}}
	composer, err := newComposer()
	if err != nil {
		return nil, err
	}
	made.composer = composer
	for _, size := range options.Panels {
		picture, err := newTarget(size.Width, size.Height, false)
		if err != nil {
			made.delete()
			return nil, err
		}
		made.pictures = append(made.pictures, picture)
		made.panels = append(made.panels, openxr.PanelWithTarget(size.Width, size.Height, func() (gles.Framebuffer, error) {
			return gles.Framebuffer(picture.framebuffer), nil
		}))
	}
	made.openPads()
	window.SetScrollCallback(func(_ *glfw.Window, _, notches float64) { made.wheel.Add(notches) })
	return made, nil
}

func (made *loop) delete() {
	if made.pads != nil {
		made.pads.Close()
	}
	for _, eye := range made.eyes {
		if eye != nil {
			eye.delete()
		}
	}
	for _, picture := range made.pictures {
		picture.delete()
	}
}

func (made *loop) run(step func(app.Frame) bool) error {
	last := -1.0
	for !made.window.ShouldClose() {
		glfw.PollEvents()
		now := glfw.GetTime()
		seconds := 0.0
		if last >= 0 {
			seconds = math.Min(now-last, longestFrame)
		}
		last = now
		frame, err := made.frame(seconds)
		if err != nil {
			return err
		}
		if !step(frame) {
			return nil
		}
		if frame.ShouldRender {
			if err := made.show(frame); err != nil {
				return err
			}
		}
		made.state.Feed().EndFrame()
		made.window.SetTitle(made.haptics.title(made.settings.Title))
	}
	return nil
}

// frame reads the window into the frame a headset would hand out.
func (made *loop) frame(seconds float64) (app.Frame, error) {
	window := made.window
	if window.GetKey(glfw.KeyEscape) == glfw.Press {
		window.SetShouldClose(true)
	}
	view := window.GetKey(glfw.KeyV) == glfw.Press
	if view && !made.viewWas {
		made.sideBySide = !made.sideBySide
	}
	made.viewWas = view
	made.look(seconds)
	pads := made.readPads(made.head.Pose())
	LookWithPads(&made.head, &pads, seconds)

	width, height := window.GetFramebufferSize()
	render := width > 0 && height > 0 && window.GetAttrib(glfw.Iconified) == 0
	eyeWidth := width
	if made.sideBySide {
		eyeWidth = width / 2
	}
	if render {
		if err := made.makeEyes(eyeWidth, height); err != nil {
			return app.Frame{}, err
		}
	}
	head := made.head.Pose()
	eyes := Eyes(head, max(eyeWidth, 1), max(height, 1))
	for index := range eyes {
		if made.eyes[index] != nil {
			eyes[index].Target = gles.Framebuffer(made.eyes[index].framebuffer)
		}
	}

	keys := made.keys(seconds)
	eye, x, y := made.cursor(eyes)
	// The keys and the mouse are the controllers only while the window has them; the pads always,
	// as the window keeps no hold on them (it may well not be in front while they are held).
	windowFocused := window.GetAttrib(glfw.Focused) == 1
	if !windowFocused {
		keys = Keys{PalmUp: keys.PalmUp}
	}
	controls := keys.Controls(LeftHand(head, keys.PalmUp), RightHand(head, eye, x, y, made.panels))
	controls = WithPads(controls, pads, made.padsWere, head)
	focused := windowFocused || pads[gamepad.LeftHand].Connected || pads[gamepad.RightHand].Connected
	controls.Apply(made.state)
	return app.Frame{Input: made.state, Head: head, HeadTracked: true,
		Hands: [2]openxr.Hand{controls.Left, controls.Right}, Controls: controls,
		Seconds: seconds, Focused: focused, ShouldRender: render, Eyes: eyes,
		Functions: desktopgl.Functions{}, Panels: made.panels, Haptics: made.haptics}, nil
}

// look turns the head: by the mouse while its right button is held, and by the arrows.
func (made *loop) look(seconds float64) {
	window := made.window
	held := window.GetMouseButton(glfw.MouseButtonRight) == glfw.Press
	x, y := window.GetCursorPos()
	switch {
	case held && !made.looking:
		window.SetInputMode(glfw.CursorMode, glfw.CursorDisabled)
		made.lookX, made.lookY = window.GetCursorPos()
	case held:
		made.head.Turn(-(x-made.lookX)*radiansPerPixel, -(y-made.lookY)*radiansPerPixel)
		made.lookX, made.lookY = x, y
	case made.looking:
		window.SetInputMode(glfw.CursorMode, glfw.CursorNormal)
		width, height := window.GetSize()
		window.SetCursorPos(float64(width)/2, float64(height)/2)
	}
	made.looking = held
	turn := seconds * radiansPerSecond
	made.head.Turn(turn*keyAxis(window, glfw.KeyRight, glfw.KeyLeft), turn*keyAxis(window, glfw.KeyDown, glfw.KeyUp))
}

func keyAxis(window *glfw.Window, negative, positive glfw.Key) float64 {
	return float64(axis(window.GetKey(negative) == glfw.Press, window.GetKey(positive) == glfw.Press))
}

// keys reads the keys standing for the controllers' (Keys).
func (made *loop) keys(seconds float64) Keys {
	window := made.window
	down := func(key glfw.Key) bool { return window.GetKey(key) == glfw.Press }
	menu := down(glfw.KeyM)
	keys := Keys{
		Forward: down(glfw.KeyW), Back: down(glfw.KeyS), Left: down(glfw.KeyA), Right: down(glfw.KeyD),
		TurnLeft: down(glfw.KeyQ), TurnRight: down(glfw.KeyE), Up: down(glfw.KeyR), Down: down(glfw.KeyF),
		Trigger: window.GetMouseButton(glfw.MouseButtonLeft) == glfw.Press, Grip: down(glfw.KeyG),
		A: down(glfw.KeySpace), B: down(glfw.KeyB), X: down(glfw.KeyX), Y: down(glfw.KeyY),
		LeftStickClick: down(glfw.KeyL), RightStickClick: down(glfw.KeyK),
		Menu: menu && !made.menuWas, PalmUp: down(glfw.KeyTab),
		Scroll: made.wheel.Push(seconds),
	}
	made.menuWas = menu
	return keys
}

// cursor is the eye the cursor is over and where in its picture, from 0 to 1 from its top-left:
// the middle while the mouse turns the head, or while a capture is made.
func (made *loop) cursor(eyes [2]openxr.Eye) (openxr.Eye, float32, float32) {
	if made.looking || made.settings.Capture != "" {
		return eyes[0], 0.5, 0.5
	}
	width, height := made.window.GetSize()
	x, y := made.window.GetCursorPos()
	across, down := float32(x)/float32(max(width, 1)), float32(y)/float32(max(height, 1))
	if !made.sideBySide {
		return eyes[0], across, down
	}
	if across < 0.5 {
		return eyes[0], across * 2, down
	}
	return eyes[1], across*2 - 1, down
}

// makeEyes makes the eyes' pictures width × height pixels, again when the window changed size.
func (made *loop) makeEyes(width, height int) error {
	for index, eye := range made.eyes {
		if eye != nil && eye.width == width && eye.height == height {
			continue
		}
		if eye != nil {
			eye.delete()
		}
		made.eyes[index] = nil
		picture, err := newTarget(width, height, true)
		if err != nil {
			return err
		}
		made.eyes[index] = picture
	}
	return nil
}

// show lays the panels over the eyes and shows them in the window.
func (made *loop) show(frame app.Frame) error {
	for _, eye := range frame.Eyes {
		made.composer.compose(eye, made.panels, made.pictures)
	}
	width, height := made.window.GetFramebufferSize()
	present(made.eyes, width, height, made.sideBySide)
	made.frames++
	if made.settings.Capture != "" && made.frames == made.settings.CaptureFrame {
		if err := capture(made.settings.Capture, width, height); err != nil {
			return err
		}
		log.Printf("FVR: captura guardada en %s", made.settings.Capture)
		made.window.SetShouldClose(true)
	}
	made.window.SwapBuffers()
	return nil
}

// openPads starts reading game pads with motion sensors, when SDL3 is installed: one for each
// hand (gamepad.Reader).
func (made *loop) openPads() {
	pads, err := gamepad.Open()
	if err != nil {
		log.Printf("FVR: sin mandos con giroscopio: %v", err)
		return
	}
	made.pads = pads
	made.haptics.rumble = pads.Rumble
}

// handNames name the hands, as the program tells the player.
var handNames = [2]string{gamepad.LeftHand: "izquierda", gamepad.RightHand: "derecha"}

// recenterNames name the button that points each kind of pad ahead.
var recenterNames = map[gamepad.Kind]string{gamepad.Whole: "−", gamepad.LeftJoyCon: "Captura", gamepad.RightJoyCon: "Home"}

// readPads reads the pads, says when one comes or goes, and points a pad where the head looks when
// its recenter button goes down.
func (made *loop) readPads(head graphics.Pose) [2]gamepad.State {
	if made.pads == nil {
		return [2]gamepad.State{}
	}
	made.padsWere = made.padsNow
	made.padsNow = made.pads.Read()
	for hand, pad := range made.padsNow {
		was := made.padsWere[hand]
		switch {
		case pad.Connected && !was.Connected:
			log.Printf("FVR: %s conectado: es la mano %s. Apúntalo al frente y pulsa %s para centrarlo.",
				pad.Name, handNames[hand], recenterNames[pad.Kind])
		case !pad.Connected && was.Connected:
			log.Printf("FVR: %s desconectado: la mano %s vuelve al teclado y al ratón.", was.Name, handNames[hand])
		}
		if pad.Pressed(pad.RecenterButton(), was) {
			made.pads.Recenter(hand, float64(place.HeadYaw(head)))
		}
	}
	return made.padsNow
}

// vibrations stand for the controllers' haptics: a hand that shakes shows in the window's title
// for a moment, and the pad in that hand, if there is one, shakes.
type vibrations struct {
	until  [2]time.Time
	rumble func(hand int, amplitude float32, duration time.Duration) // the pads', with SDL3
}

func (shaking *vibrations) Vibrate(hand int, amplitude float32, duration time.Duration) {
	if hand < 0 || hand > 1 || amplitude <= 0 {
		return
	}
	shaking.until[hand] = time.Now().Add(max(duration, vibrationTitleFor))
	if shaking.rumble != nil {
		shaking.rumble(hand, amplitude, duration)
	}
}

func (shaking *vibrations) title(name string) string {
	now := time.Now()
	left, right := now.Before(shaking.until[app.LeftHand]), now.Before(shaking.until[app.RightHand])
	switch {
	case left && right:
		return name + " · vibran las dos manos"
	case left:
		return name + " · vibra la mano izquierda"
	case right:
		return name + " · vibra la mano derecha"
	}
	return name
}
