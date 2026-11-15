// Package gamepad reads game pads with motion sensors (a pair of Joy-Con, a Switch Pro Controller,
// an 8BitDo pad in its Switch mode) through SDL3: their sticks, triggers and buttons, and where
// each points, from its gyroscope and accelerometer (motion.Orientation). Up to two are read, one
// for each hand: a left Joy-Con is the left hand, a right Joy-Con the right one, and a whole pad
// the right hand, or the left when the right is taken. They can come and go while the program runs.
//
// The Joy-Con are read apart, not joined into one pad, and upright (SDL's vertical mode): each is
// held as a headset controller is, pointing ahead along its length, and has the half of a pad it
// carries: the left one the left stick, the d-pad, L, ZL and −; the right one the right stick, the
// face buttons, R, ZR and +.
//
// SDL3 is loaded from the system when Open is called, so a program builds without it and runs
// without pads.
package gamepad

import (
	"fmt"
	"math"
	"time"

	"github.com/akrck02/FVR/motion"
	"github.com/akrck02/go-renderer/graphics"
)

// The hands the pads are held in, as Read hands them out.
const (
	LeftHand  = 0
	RightHand = 1
)

// The pads' sticks and triggers in State, as SDL numbers its axes.
const (
	LeftX = iota
	LeftY
	RightX
	RightY
	LeftTrigger
	RightTrigger
	axisCount
)

// The pads' buttons in State, as SDL numbers them: by where they are, so South is the bottom face
// button whatever letter it carries.
const (
	South = iota
	East
	West
	North
	Back // − on a Switch pad
	Guide
	Start // + on a Switch pad
	LeftStick
	RightStick
	LeftShoulder
	RightShoulder
	Up
	Down
	Left
	Right
	Capture
	buttonCount
)

// Kind is what a pad is, which says which hand holds it and which buttons it has.
type Kind int

const (
	// Whole is a pad with everything: a Pro Controller, an 8BitDo pad.
	Whole Kind = iota + 1
	LeftJoyCon
	RightJoyCon
)

// SDL's types of pad (SDL_GamepadType) for the Joy-Con.
const (
	sdlLeftJoyCon  = 8
	sdlRightJoyCon = 9
)

// longestSensorGap is the longest time between two gyroscope readings the filter is turned by: a
// pad that went quiet (asleep, out of reach) starts again from its next reading.
const longestSensorGap = 0.1

// State is a pad as it was when last read.
type State struct {
	Connected bool
	Name      string
	Kind      Kind
	// Axes are the sticks, -1 to 1 with up and right positive, and the triggers, 0 to 1.
	Axes [axisCount]float32
	// Buttons are held: index them with South, East, …
	Buttons [buttonCount]bool
	// Rotation is where the pad points in the room: -Z ahead, as a headset controller's aim.
	Rotation graphics.Quaternion
}

// RecenterButton is the button that points the pad ahead again: Capture on a left Joy-Con, Home on
// a right one, − on a whole pad.
func (state State) RecenterButton() int {
	switch state.Kind {
	case LeftJoyCon:
		return Capture
	case RightJoyCon:
		return Guide
	}
	return Back
}

// MenuButton is the button that stands for the headset's menu button: − on a left Joy-Con, + on
// the others.
func (state State) MenuButton() int {
	if state.Kind == LeftJoyCon {
		return Back
	}
	return Start
}

// Pressed tells whether a button went down between an earlier state and this one.
func (state State) Pressed(button int, earlier State) bool {
	return state.Buttons[button] && !earlier.Buttons[button]
}

// pad is one pad being read.
type pad struct {
	handle      uintptr
	id          uint32
	name        string
	kind        Kind
	orientation motion.Orientation
	pull        [3]float32 // the last accelerometer reading
	lastTurn    uint64     // when the last gyroscope reading was taken, in nanoseconds
}

// Reader is SDL3, reading a pad for each hand.
type Reader struct {
	library *sdl
	hands   [2]*pad
}

// Open loads SDL3 and starts listening for pads. It fails when SDL3 is not installed (ErrNoSDL).
// It must be called from the thread the reader will be used on.
func Open() (*Reader, error) {
	library, err := loadSDL()
	if err != nil {
		return nil, err
	}
	library.setHint("SDL_JOYSTICK_HIDAPI", "1")
	library.setHint("SDL_JOYSTICK_HIDAPI_COMBINE_JOY_CONS", "0")
	library.setHint("SDL_JOYSTICK_HIDAPI_VERTICAL_JOY_CONS", "1")
	if !library.initialize(initGamepad | initSensor) {
		return nil, fmt.Errorf("gamepad: SDL does not start: %s", library.lastError())
	}
	return &Reader{library: library}, nil
}

// Close lets go of the pads and SDL.
func (reader *Reader) Close() {
	for hand, held := range reader.hands {
		if held != nil {
			reader.library.closeGamepad(held.handle)
			reader.hands[hand] = nil
		}
	}
	reader.library.quit()
}

// Read takes what the pads sent since the last read (pads coming and going, and every sensor
// reading, each turning its pad's orientation) and returns the pads as they are now, by hand
// (LeftHand, RightHand).
func (reader *Reader) Read() [2]State {
	library := reader.library
	var bytes [eventSize]byte
	for library.pollEvent(&bytes) {
		reader.handle(readEvent(&bytes))
	}
	var states [2]State
	for hand, held := range reader.hands {
		if held == nil {
			continue
		}
		state := State{Connected: true, Name: held.name, Kind: held.kind, Rotation: held.orientation.Rotation()}
		for axis := range axisCount {
			value := float32(library.axis(held.handle, int32(axis))) / math.MaxInt16
			state.Axes[axis] = float32(math.Max(-1, math.Min(1, float64(value))))
		}
		state.Axes[LeftY], state.Axes[RightY] = -state.Axes[LeftY], -state.Axes[RightY] // SDL's sticks are down positive
		for button := range buttonCount {
			state.Buttons[button] = library.button(held.handle, int32(button))
		}
		states[hand] = state
	}
	return states
}

func (reader *Reader) handle(happened event) {
	switch happened.kind {
	case eventGamepadAdded:
		reader.open(happened.which)
	case eventGamepadRemoved:
		for hand, held := range reader.hands {
			if held != nil && held.id == happened.which {
				reader.library.closeGamepad(held.handle)
				reader.hands[hand] = nil
			}
		}
	case eventSensorUpdate:
		for _, held := range reader.hands {
			if held != nil && held.id == happened.which {
				held.sense(happened)
			}
		}
	}
}

// open opens a pad that has a gyroscope and an accelerometer, switches them on and gives it its
// hand; a pad without them, or with no hand free for it, is left alone.
func (reader *Reader) open(id uint32) {
	library := reader.library
	handle := library.openGamepad(id)
	if handle == 0 {
		return
	}
	kind := kindOf(library.gamepadType(handle))
	hand, free := reader.handFor(kind)
	if !free || !library.hasSensor(handle, sensorGyroscope) || !library.hasSensor(handle, sensorAccelerometer) {
		library.closeGamepad(handle)
		return
	}
	library.enableSensor(handle, sensorGyroscope, true)
	library.enableSensor(handle, sensorAccelerometer, true)
	reader.hands[hand] = &pad{handle: handle, id: id, name: library.gamepadName(handle), kind: kind}
}

func kindOf(sdlType int32) Kind {
	switch sdlType {
	case sdlLeftJoyCon:
		return LeftJoyCon
	case sdlRightJoyCon:
		return RightJoyCon
	}
	return Whole
}

// handFor is the hand a pad of a kind goes to, and whether it is free: a Joy-Con's own hand, or for
// a whole pad the right hand, else the left.
func (reader *Reader) handFor(kind Kind) (int, bool) {
	switch kind {
	case LeftJoyCon:
		return LeftHand, reader.hands[LeftHand] == nil
	case RightJoyCon:
		return RightHand, reader.hands[RightHand] == nil
	}
	if reader.hands[RightHand] == nil {
		return RightHand, true
	}
	return LeftHand, reader.hands[LeftHand] == nil
}

// sense turns the orientation by a gyroscope reading, with the last accelerometer one.
func (held *pad) sense(happened event) {
	if happened.sensor == sensorAccelerometer {
		held.pull = happened.data
		return
	}
	if happened.sensor != sensorGyroscope || held.pull == [3]float32{} {
		return
	}
	seconds := 0.0
	if held.lastTurn != 0 && happened.timestamp > held.lastTurn {
		seconds = float64(happened.timestamp-held.lastTurn) / float64(time.Second)
	}
	held.lastTurn = happened.timestamp
	if seconds > longestSensorGap {
		seconds = 0
	}
	held.orientation.Update(happened.data, held.pull, seconds)
}

// Recenter turns the pad in a hand so it points toward yaw (motion.Orientation.Recenter).
func (reader *Reader) Recenter(hand int, yaw float64) {
	if held := reader.hands[hand]; held != nil {
		held.orientation.Recenter(yaw)
	}
}

// Rumble shakes the pad in a hand, amplitude from 0 to 1, for a time.
func (reader *Reader) Rumble(hand int, amplitude float32, duration time.Duration) {
	held := reader.hands[hand]
	if held == nil {
		return
	}
	strength := uint16(math.Max(0, math.Min(1, float64(amplitude))) * math.MaxUint16)
	reader.library.rumble(held.handle, strength, strength, uint32(duration.Milliseconds()))
}
