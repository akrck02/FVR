// Package motion turns a game pad's motion sensors into a hand: where the pad points (a filter
// that joins its gyroscope and its accelerometer) and where the hand holding it is (an arm model).
// It is plain arithmetic, with no device in it: the gamepad package reads the sensors.
//
// The sensors are read as SDL gives them, for a pad held in front of you: X to the right, Y up and
// Z toward you, so -Z is where the pad points, as a headset controller's aim. The gyroscope is in
// radians a second, counter-clockwise around each axis; the accelerometer in metres a second
// squared, reading the pull of gravity upward at rest.
package motion

import (
	"math"

	"github.com/akrck02/go-meta-quest/place"
	"github.com/akrck02/go-renderer/graphics"
)

// Gravity is the accelerometer's reading at rest, in metres a second squared.
const Gravity = 9.80665

// How the filter weighs the accelerometer against the gyroscope, and when it takes the pad as
// lying still to learn the gyroscope's bias.
const (
	// correction is how fast the accelerometer pulls the tilt back (Mahony's proportional gain):
	// higher follows gravity sooner, and shakes with every jolt of the hand.
	correction = 1.0
	// trustedPull is how far the accelerometer's reading may be from gravity and still be taken as
	// showing where up is: a swing adds its own acceleration.
	trustedPull = 0.25 * Gravity
	// stillTurn and stillPull are how little the pad turns (radians a second) and how close to
	// gravity it pulls for it to be taken as lying still.
	stillTurn = 0.08
	stillPull = 0.05 * Gravity
	// stillSettle is how long it must be still before its gyroscope's reading is taken as bias,
	// and biasSeconds how slowly the bias follows it then.
	stillSettle = 0.5
	biasSeconds = 2.0
)

// Orientation follows where a pad points from its sensors: the gyroscope turns it, and gravity,
// in the accelerometer, keeps its tilt from drifting (a Mahony filter). Nothing keeps its yaw:
// Recenter puts it back where the player says ahead is. While the pad lies still the filter learns
// the gyroscope's bias, its reading at rest, and takes it off.
type Orientation struct {
	rotation [4]float64 // w, x, y, z: from the pad's axes to the room's
	bias     [3]float64
	still    float64 // seconds the pad has been still
	started  bool
}

// Rotation is the pad's orientation in the room: a pad lying flat, pointing ahead, is the
// identity.
func (filter *Orientation) Rotation() graphics.Quaternion {
	r := filter.rotation
	if !filter.started {
		return graphics.IdentityQuaternion()
	}
	return graphics.Quaternion{W: float32(r[0]), X: float32(r[1]), Y: float32(r[2]), Z: float32(r[3])}
}

// Bias is the gyroscope's reading at rest the filter has learned, in radians a second.
func (filter *Orientation) Bias() [3]float64 { return filter.bias }

// Update moves the orientation by a reading of the sensors seconds after the last one.
func (filter *Orientation) Update(gyroscope, accelerometer [3]float32, seconds float64) {
	turn := [3]float64{float64(gyroscope[0]), float64(gyroscope[1]), float64(gyroscope[2])}
	pull := [3]float64{float64(accelerometer[0]), float64(accelerometer[1]), float64(accelerometer[2])}
	strength := length(pull)
	if !filter.started {
		if strength == 0 {
			return
		}
		filter.rotation = upright(scaled(pull, 1/strength))
		filter.started = true
		return
	}
	if seconds <= 0 {
		return
	}
	filter.learnBias(turn, strength, seconds)
	for axis := range turn {
		turn[axis] -= filter.bias[axis]
	}
	if math.Abs(strength-Gravity) < trustedPull {
		up := scaled(pull, 1/strength)
		estimated := rotate(conjugate(filter.rotation), [3]float64{0, 1, 0})
		error := cross(up, estimated)
		for axis := range turn {
			turn[axis] += correction * error[axis]
		}
	}
	filter.rotation = normalized(integrate(filter.rotation, turn, seconds))
}

// learnBias takes the gyroscope's reading as its bias, slowly, once the pad has lain still a moment.
func (filter *Orientation) learnBias(turn [3]float64, strength, seconds float64) {
	if length(turn) > stillTurn+length(filter.bias) || math.Abs(strength-Gravity) > stillPull {
		filter.still = 0
		return
	}
	filter.still += seconds
	if filter.still < stillSettle {
		return
	}
	share := math.Min(1, seconds/biasSeconds)
	for axis := range turn {
		filter.bias[axis] += (turn[axis] - filter.bias[axis]) * share
	}
}

// Recenter turns the orientation around up so the pad points toward yaw (as place.HeadYaw gives
// it), keeping its tilt: the player points ahead and says so.
func (filter *Orientation) Recenter(yaw float64) {
	if !filter.started {
		return
	}
	ahead := rotate(filter.rotation, [3]float64{0, 0, -1})
	if math.Hypot(ahead[0], ahead[2]) < 1e-3 {
		ahead = rotate(filter.rotation, [3]float64{0, 1, 0}) // pointing straight up or down: the top of the pad
	}
	turn := yaw - float64(place.YawFacing(ahead[0], ahead[2]))
	around := [4]float64{math.Cos(turn / 2), 0, math.Sin(turn / 2), 0}
	filter.rotation = normalized(multiply(around, filter.rotation))
}

// upright is the rotation of a pad whose accelerometer reads up along it, pointing as far ahead as
// its tilt lets it: the shortest turn from up to the room's up.
func upright(up [3]float64) [4]float64 {
	axis := cross(up, [3]float64{0, 1, 0})
	cosine := up[1]
	if cosine < -0.9999 { // upside down: half a turn around the pad's right
		return [4]float64{0, 1, 0, 0}
	}
	half := math.Sqrt((1 + cosine) * 2)
	return normalized([4]float64{half / 2, axis[0] / half, axis[1] / half, axis[2] / half})
}

// integrate turns a rotation by an angular speed (in its own axes) for a time.
func integrate(rotation [4]float64, turn [3]float64, seconds float64) [4]float64 {
	angle := length(turn) * seconds
	if angle == 0 {
		return rotation
	}
	axis := scaled(turn, 1/length(turn))
	sine := math.Sin(angle / 2)
	step := [4]float64{math.Cos(angle / 2), axis[0] * sine, axis[1] * sine, axis[2] * sine}
	return multiply(rotation, step)
}

func multiply(a, b [4]float64) [4]float64 {
	return [4]float64{
		a[0]*b[0] - a[1]*b[1] - a[2]*b[2] - a[3]*b[3],
		a[0]*b[1] + a[1]*b[0] + a[2]*b[3] - a[3]*b[2],
		a[0]*b[2] - a[1]*b[3] + a[2]*b[0] + a[3]*b[1],
		a[0]*b[3] + a[1]*b[2] - a[2]*b[1] + a[3]*b[0],
	}
}

func conjugate(rotation [4]float64) [4]float64 {
	return [4]float64{rotation[0], -rotation[1], -rotation[2], -rotation[3]}
}

func rotate(rotation [4]float64, vector [3]float64) [3]float64 {
	turned := multiply(multiply(rotation, [4]float64{0, vector[0], vector[1], vector[2]}), conjugate(rotation))
	return [3]float64{turned[1], turned[2], turned[3]}
}

func normalized(rotation [4]float64) [4]float64 {
	size := math.Sqrt(rotation[0]*rotation[0] + rotation[1]*rotation[1] + rotation[2]*rotation[2] + rotation[3]*rotation[3])
	if size == 0 {
		return [4]float64{1, 0, 0, 0}
	}
	return [4]float64{rotation[0] / size, rotation[1] / size, rotation[2] / size, rotation[3] / size}
}

func cross(a, b [3]float64) [3]float64 {
	return [3]float64{a[1]*b[2] - a[2]*b[1], a[2]*b[0] - a[0]*b[2], a[0]*b[1] - a[1]*b[0]}
}

func length(vector [3]float64) float64 {
	return math.Sqrt(vector[0]*vector[0] + vector[1]*vector[1] + vector[2]*vector[2])
}

func scaled(vector [3]float64, factor float64) [3]float64 {
	return [3]float64{vector[0] * factor, vector[1] * factor, vector[2] * factor}
}

// The arm the pad is held in, from the body under the head: the elbow stays by the side, and the
// forearm turns with the pad (a simple arm model, as phone headsets used for their one controller).
const (
	elbowAside    = 0.2  // metres to the side of the head
	elbowBelow    = 0.5  // metres under the eyes
	elbowAhead    = 0.05 // metres ahead of the eyes
	forearmLength = 0.3  // metres from the elbow to the hand
)

// Hand is where a hand holding the pad is and how it is turned, for a pad turned by rotation in a
// player whose head is at head: side is 1 for the right arm, -1 for the left.
func Hand(head graphics.Pose, rotation graphics.Quaternion, side float32) graphics.Pose {
	body := graphics.QuaternionAroundY(float64(place.HeadYaw(head)))
	elbow := body.Rotate(graphics.Vector{side * elbowAside, -elbowBelow, -elbowAhead, 0})
	forearm := rotation.Rotate(graphics.Vector{0, 0, -forearmLength, 0})
	position := graphics.Vector{head.Position[0] + elbow[0] + forearm[0], head.Position[1] + elbow[1] + forearm[1],
		head.Position[2] + elbow[2] + forearm[2], 1}
	return graphics.Pose{Orientation: rotation, Position: position}
}
