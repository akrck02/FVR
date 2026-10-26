package headset

import (
	"math"

	"github.com/akrck02/go-meta-quest/mirror"
	"github.com/akrck02/go-meta-quest/place"
	"github.com/akrck02/go-meta-quest/pointer"
	"github.com/akrck02/go-meta-quest/selftest"
	"github.com/akrck02/go-renderer/graphics"
	"github.com/akrck02/go-renderer/openxr"
)

// farMetres is how far ahead the right hand points when the cursor is over no panel: far enough
// that the hand and the eye look at the same thing.
const farMetres = 10

// Keys are the keys and buttons standing for the controllers' own, as the window last read them.
// The sticks are pushed all the way while their keys are held.
type Keys struct {
	Forward, Back, Left, Right bool // the left stick: W, S, A, D
	TurnLeft, TurnRight        bool // the right stick sideways: Q, E
	Up, Down                   bool // the right stick up and down: R, F
	Trigger, Grip              bool // the right hand's: the mouse's left button, G
	A, B, X, Y                 bool // the face buttons, on the keys of their letters
	LeftStickClick             bool // L
	RightStickClick            bool // K
	Menu                       bool // M went down since the last frame
	PalmUp                     bool // Tab: the left hand's palm turned to the face
	// Scroll pushes the right stick up (positive) or down, from the mouse's wheel; it adds to Up
	// and Down.
	Scroll float32
}

// Controls are the controllers the keys make, holding the hands given.
func (keys Keys) Controls(left, right openxr.Hand) openxr.Controls {
	left.StickX, left.StickY = axis(keys.Left, keys.Right), axis(keys.Back, keys.Forward)
	left.Upper, left.Lower, left.StickClick = keys.Y, keys.X, keys.LeftStickClick
	right.StickX = axis(keys.TurnLeft, keys.TurnRight)
	right.StickY = float32(math.Max(-1, math.Min(1, float64(axis(keys.Down, keys.Up)+keys.Scroll))))
	right.Upper, right.Lower, right.StickClick = keys.B, keys.A, keys.RightStickClick
	right.Trigger, right.Squeeze = pressure(keys.Trigger), pressure(keys.Grip)
	return openxr.Controls{Left: left, Right: right, Menu: keys.Menu}
}

// axis is a stick's axis from the keys at its two ends.
func axis(negative, positive bool) float32 {
	return pressure(positive) - pressure(negative)
}

// pressure is a trigger or a grip all the way in while its key is held.
func pressure(held bool) float32 {
	if held {
		return 1
	}
	return 0
}

// RightHand is the right controller the mouse makes: held at rest beside the head (where
// selftest.HandBeside has it) and pointing at what the cursor is over in the eye's picture, x and y
// from 0 to 1 from its top-left corner: the nearest shown panel, or far ahead. So the beam leaves
// the hand and lands under the cursor.
func RightHand(head graphics.Pose, eye openxr.Eye, x, y float32, panels []*openxr.Panel) openxr.Hand {
	origin := eye.Pose.Position
	origin[3] = 1
	sight := pointer.Ray{Origin: origin, Direction: mirror.Direction(eye, x, y)}
	distance := float32(farMetres)
	for _, panel := range panels {
		if panel == nil || !panel.Shown {
			continue
		}
		if hit, hits := pointer.HitPanel(sight, panel); hits && hit.Distance < distance {
			distance = hit.Distance
		}
	}
	aim := pointer.AimAt(selftest.HandBeside(head), sight.At(distance))
	return openxr.Hand{Tracked: true, Grip: aim, Aim: aim}
}

// LeftHand is the left controller: at rest low and to the left of the head, pointing ahead, or with
// its palm turned to the face (selftest.PalmToFace), as a program's hand menu asks.
func LeftHand(head graphics.Pose, palmUp bool) openxr.Hand {
	if palmUp {
		return selftest.PalmToFace(head)
	}
	turn := graphics.QuaternionAroundY(float64(place.HeadYaw(head)))
	offset := turn.Rotate(graphics.Vector{-0.18, -0.35, -0.2, 0})
	origin := graphics.Vector{head.Position[0] + offset[0], head.Position[1] + offset[1], head.Position[2] + offset[2], 1}
	ahead := turn.Rotate(graphics.Vector{0, 0, -1, 0})
	aim := pointer.AimAt(origin, graphics.Vector{origin[0] + ahead[0], origin[1], origin[2] + ahead[2], 1})
	return openxr.Hand{Tracked: true, Grip: aim, Aim: aim}
}

// Wheel turns the mouse's wheel into the right stick, as a controller scrolls: each notch pushes
// the stick all the way for as long as pointer.ScrollNotchesPerSecond takes to scroll one.
type Wheel struct {
	notches float64 // still to scroll: up positive
}

// Add adds the notches the wheel turned (up positive).
func (wheel *Wheel) Add(notches float64) { wheel.notches += notches }

// Push is the stick for a frame of seconds, and takes from the notches what it scrolls.
func (wheel *Wheel) Push(seconds float64) float32 {
	if wheel.notches == 0 || seconds <= 0 {
		return 0
	}
	step := pointer.ScrollNotchesPerSecond * seconds
	if math.Abs(wheel.notches) <= step {
		push := wheel.notches / step
		wheel.notches = 0
		return float32(math.Copysign(math.Max(math.Abs(push), 0.5), push)) // past the stick's resting zone
	}
	wheel.notches -= math.Copysign(step, wheel.notches)
	return float32(math.Copysign(1, wheel.notches))
}
