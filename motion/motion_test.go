package motion

import (
	"math"
	"testing"

	"github.com/akrck02/go-renderer/graphics"
)

const sample = 1.0 / 200 // seconds between readings, as a pad over Bluetooth sends them

func pointing(filter *Orientation) graphics.Vector {
	return filter.Rotation().Rotate(graphics.Vector{0, 0, -1, 0})
}

func feed(filter *Orientation, gyroscope, accelerometer [3]float32, seconds float64) {
	for elapsed := 0.0; elapsed < seconds; elapsed += sample {
		filter.Update(gyroscope, accelerometer, sample)
	}
}

func close(a, b, within float32) bool { return math.Abs(float64(a-b)) < float64(within) }

var flat = [3]float32{0, Gravity, 0}

func TestAPadLyingFlatPointsAhead(test *testing.T) {
	var filter Orientation
	feed(&filter, [3]float32{}, flat, 1)
	ahead := pointing(&filter)
	if !close(ahead[2], -1, 1e-3) {
		test.Fatalf("pointing %v", ahead)
	}
}

func TestAPadTiltedUpPointsUpFromItsFirstReading(test *testing.T) {
	var filter Orientation
	tilt := 30 * math.Pi / 180
	up := [3]float32{0, float32(Gravity * math.Cos(tilt)), float32(-Gravity * math.Sin(tilt))}
	filter.Update([3]float32{}, up, 0)
	ahead := pointing(&filter)
	if !close(ahead[1], float32(math.Sin(tilt)), 1e-3) || !close(ahead[2], float32(-math.Cos(tilt)), 1e-3) {
		test.Fatalf("pointing %v, want 30° up", ahead)
	}
}

func TestTheGyroscopeTurnsThePad(test *testing.T) {
	var filter Orientation
	filter.Update([3]float32{}, flat, 0)
	feed(&filter, [3]float32{0, 1, 0}, flat, 1) // a radian to the left, around up
	ahead := pointing(&filter)
	if !close(ahead[0], float32(-math.Sin(1)), 0.02) || !close(ahead[2], float32(-math.Cos(1)), 0.02) {
		test.Fatalf("pointing %v, want a radian to the left", ahead)
	}
}

func TestGravityPullsADriftedTiltBack(test *testing.T) {
	var filter Orientation
	filter.Update([3]float32{}, flat, 0)
	feed(&filter, [3]float32{0.5, 0, 0}, flat, 0.4) // a gyroscope that lies: the pad tilts up
	if pointing(&filter)[1] < 0.05 {
		test.Fatal("the lying gyroscope did not tilt the pad")
	}
	feed(&filter, [3]float32{}, flat, 6)
	if ahead := pointing(&filter); !close(ahead[1], 0, 0.02) {
		test.Fatalf("still tilted: pointing %v", ahead)
	}
}

func TestAPadLyingStillLearnsItsGyroscopesBias(test *testing.T) {
	var filter Orientation
	filter.Update([3]float32{}, flat, 0)
	drift := [3]float32{0, 0.01, 0}
	feed(&filter, drift, flat, 10)
	if bias := filter.Bias(); math.Abs(bias[1]-0.01) > 0.002 {
		test.Fatalf("bias %v, want 0.01 around up", bias)
	}
	before := pointing(&filter)
	feed(&filter, drift, flat, 10)
	after := pointing(&filter)
	if math.Abs(math.Atan2(float64(-after[0]), float64(-after[2]))-math.Atan2(float64(-before[0]), float64(-before[2]))) > 0.02 {
		test.Fatalf("the yaw still drifts: %v then %v", before, after)
	}
}

func TestAMovingPadTeachesNoBias(test *testing.T) {
	var filter Orientation
	filter.Update([3]float32{}, flat, 0)
	feed(&filter, [3]float32{0, 2, 0}, flat, 3)
	if bias := filter.Bias(); bias != [3]float64{} {
		test.Fatalf("learned %v while turning", bias)
	}
}

func TestRecenteringPointsThePadWhereTheHeadLooksAndKeepsItsTilt(test *testing.T) {
	var filter Orientation
	tilt := 20 * math.Pi / 180
	filter.Update([3]float32{}, [3]float32{0, float32(Gravity * math.Cos(tilt)), float32(-Gravity * math.Sin(tilt))}, 0)
	filter.Recenter(-math.Pi / 2) // the head looks to the right
	ahead := pointing(&filter)
	if !close(ahead[0], float32(math.Cos(tilt)), 0.03) || !close(ahead[1], float32(math.Sin(tilt)), 0.03) {
		test.Fatalf("pointing %v, want to the right and 20° up", ahead)
	}
}

func TestTheHandIsBesideTheBodyAndRisesAsThePadPointsUp(test *testing.T) {
	head := graphics.Pose{Orientation: graphics.IdentityQuaternion(), Position: graphics.Vector{0, 1.6, 0, 1}}
	ahead := Hand(head, graphics.IdentityQuaternion(), 1)
	if ahead.Position[0] <= 0 || ahead.Position[2] >= 0 || ahead.Position[1] >= 1.6 {
		test.Fatalf("right hand at %v", ahead.Position)
	}
	up := graphics.Quaternion{X: float32(math.Sin(math.Pi / 4)), W: float32(math.Cos(math.Pi / 4))}
	raised := Hand(head, up, 1)
	if raised.Position[1] <= ahead.Position[1] {
		test.Fatalf("pointing up the hand is at %v, not above %v", raised.Position, ahead.Position)
	}
	if left := Hand(head, graphics.IdentityQuaternion(), -1); left.Position[0] >= 0 {
		test.Fatalf("left hand at %v", left.Position)
	}
}
