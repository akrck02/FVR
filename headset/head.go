package headset

import (
	"math"

	"github.com/akrck02/go-renderer/graphics"
	"github.com/akrck02/go-renderer/openxr"
)

// The person the window stands in for: their eyes at a standing height above the floor, the usual
// distance apart, and a field of view about a headset's.
const (
	// EyeHeightMetres is how high the eyes are, standing, above the floor of the player's space.
	EyeHeightMetres = 1.6
	// EyesApartMetres is the distance between the two eyes.
	EyesApartMetres = 0.063
	// HalfHeightRadians is how far the eyes see up and down from straight ahead: the window's
	// height spans twice this.
	HalfHeightRadians = 45 * math.Pi / 180
	// mostPitch is as far up or down as the head looks: past straight up, yaw turns upside down.
	mostPitch = 85 * math.Pi / 180
)

// Head is where the head is and where it looks: it stands at the origin's eye height and turns
// by yaw (around up, left positive) and pitch (up positive), as a mouse turns it.
type Head struct {
	Position   graphics.Vector
	Yaw, Pitch float64
}

// StandingHead is a head at the middle of the player's space, at eye height, looking ahead (-Z).
func StandingHead() Head {
	return Head{Position: graphics.Vector{0, EyeHeightMetres, 0, 1}}
}

// Turn turns the head by yaw and pitch, keeping it from looking past straight up or down.
func (head *Head) Turn(yaw, pitch float64) {
	head.Yaw = math.Remainder(head.Yaw+yaw, 2*math.Pi)
	head.Pitch = math.Max(-mostPitch, math.Min(mostPitch, head.Pitch+pitch))
}

// Pose is the head as a headset tells it: turned by yaw, then tilted by pitch.
func (head Head) Pose() graphics.Pose {
	tilt := graphics.Quaternion{X: float32(math.Sin(head.Pitch / 2)), W: float32(math.Cos(head.Pitch / 2))}
	return graphics.Pose{Orientation: graphics.QuaternionAroundY(head.Yaw).Multiply(tilt), Position: head.Position}
}

// Eyes are the head's two eyes, left then right, each half the eyes' distance to its side and
// seeing a picture of width × height pixels: as far up and down as HalfHeightRadians, and as far to
// the sides as that picture's shape asks. Their targets are left for the caller to set.
func Eyes(head graphics.Pose, width, height int) [2]openxr.Eye {
	field := FieldFor(width, height)
	var eyes [2]openxr.Eye
	for index, side := range [2]float32{-1, 1} {
		offset := head.Orientation.Rotate(graphics.Vector{side * EyesApartMetres / 2, 0, 0, 0})
		position := graphics.Vector{head.Position[0] + offset[0], head.Position[1] + offset[1], head.Position[2] + offset[2], 1}
		eyes[index] = openxr.Eye{Pose: graphics.Pose{Orientation: head.Orientation, Position: position},
			Field: field, Width: width, Height: height}
	}
	return eyes
}

// FieldFor is the field of view of a picture of width × height pixels: HalfHeightRadians up and
// down, and to each side as wide as the picture is.
func FieldFor(width, height int) graphics.FieldOfView {
	aspect := 1.0
	if width > 0 && height > 0 {
		aspect = float64(width) / float64(height)
	}
	across := float32(math.Atan(math.Tan(HalfHeightRadians) * aspect))
	return graphics.FieldOfView{LeftRadians: -across, RightRadians: across,
		UpRadians: HalfHeightRadians, DownRadians: -HalfHeightRadians}
}
