package gamepad

import (
	"encoding/binary"
	"errors"
	"math"
	"runtime"

	"github.com/ebitengine/purego"
)

// The few SDL3 functions and constants the reader uses, loaded from the system's SDL3 library
// when it is opened (purego: no cgo, and no SDL needed to build).

// libraryPaths are where SDL3 is looked for, in turn.
var libraryPaths = map[string][]string{
	"darwin": {"libSDL3.dylib", "libSDL3.0.dylib", "/opt/homebrew/lib/libSDL3.dylib", "/usr/local/lib/libSDL3.dylib"},
	"linux":  {"libSDL3.so.0", "libSDL3.so"},
}

// ErrNoSDL is the error Open gives when SDL3 is not installed.
var ErrNoSDL = errors.New("gamepad: SDL3 is not installed (macOS: brew install sdl3)")

const (
	initGamepad = 0x2000
	initSensor  = 0x8000

	eventGamepadAdded   = 0x653
	eventGamepadRemoved = 0x654
	eventSensorUpdate   = 0x659

	sensorAccelerometer = 1
	sensorGyroscope     = 2

	eventSize = 128
)

// sdl is the loaded library.
type sdl struct {
	setHint        func(name, value string) bool
	initialize     func(flags uint32) bool
	quit           func()
	lastError      func() string
	pollEvent      func(event *[eventSize]byte) bool
	openGamepad    func(id uint32) uintptr
	closeGamepad   func(pad uintptr)
	gamepadName    func(pad uintptr) string
	gamepadType    func(pad uintptr) int32
	hasSensor      func(pad uintptr, sensor int32) bool
	enableSensor   func(pad uintptr, sensor int32, enabled bool) bool
	axis           func(pad uintptr, axis int32) int16
	button         func(pad uintptr, button int32) bool
	rumble         func(pad uintptr, low, high uint16, milliseconds uint32) bool
	updateGamepads func()
}

func loadSDL() (*sdl, error) {
	var library uintptr
	for _, path := range libraryPaths[runtime.GOOS] {
		if handle, err := purego.Dlopen(path, purego.RTLD_NOW|purego.RTLD_GLOBAL); err == nil {
			library = handle
			break
		}
	}
	if library == 0 {
		return nil, ErrNoSDL
	}
	made := &sdl{}
	for name, function := range map[string]any{
		"SDL_SetHint": &made.setHint, "SDL_Init": &made.initialize, "SDL_Quit": &made.quit,
		"SDL_GetError": &made.lastError, "SDL_PollEvent": &made.pollEvent,
		"SDL_OpenGamepad": &made.openGamepad, "SDL_CloseGamepad": &made.closeGamepad,
		"SDL_GetGamepadName": &made.gamepadName, "SDL_GetGamepadType": &made.gamepadType, "SDL_GamepadHasSensor": &made.hasSensor,
		"SDL_SetGamepadSensorEnabled": &made.enableSensor, "SDL_GetGamepadAxis": &made.axis,
		"SDL_GetGamepadButton": &made.button, "SDL_RumbleGamepad": &made.rumble,
		"SDL_UpdateGamepads": &made.updateGamepads,
	} {
		purego.RegisterLibFunc(function, library, name)
	}
	return made, nil
}

// event is an SDL_Event as the reader reads it: its type, and the fields of the gamepad events it
// knows (SDL_GamepadDeviceEvent, SDL_GamepadSensorEvent).
type event struct {
	kind      uint32
	which     uint32
	sensor    int32
	data      [3]float32
	timestamp uint64 // the sensor's reading, in nanoseconds
}

// readEvent reads the fields from an SDL_Event's bytes (SDL3's layout, little-endian).
func readEvent(bytes *[eventSize]byte) event {
	read := binary.LittleEndian
	return event{
		kind:   read.Uint32(bytes[0:]),
		which:  read.Uint32(bytes[16:]),
		sensor: int32(read.Uint32(bytes[20:])),
		data: [3]float32{math.Float32frombits(read.Uint32(bytes[24:])), math.Float32frombits(read.Uint32(bytes[28:])),
			math.Float32frombits(read.Uint32(bytes[32:]))},
		timestamp: read.Uint64(bytes[40:]),
	}
}
