package headset

import (
	"math"
	"testing"

	"github.com/akrck02/go-meta-quest/pointer"
	"github.com/akrck02/go-renderer/graphics"
	"github.com/akrck02/go-renderer/input"
	"github.com/akrck02/go-renderer/openxr"
)

func near(a, b float32) bool { return math.Abs(float64(a-b)) < 1e-3 }

func forward(pose graphics.Pose) graphics.Vector {
	return pose.Orientation.Rotate(graphics.Vector{0, 0, -1, 0})
}

func TestAStandingHeadLooksAheadAtEyeHeight(test *testing.T) {
	pose := StandingHead().Pose()
	ahead := forward(pose)
	if !near(pose.Position[1], EyeHeightMetres) || !near(ahead[2], -1) {
		test.Fatalf("head at %v looking %v, want eye height looking -Z", pose.Position, ahead)
	}
}

func TestTurningLeftAndUpLooksLeftAndUp(test *testing.T) {
	head := StandingHead()
	head.Turn(math.Pi/2, 0.3)
	ahead := forward(head.Pose())
	if ahead[0] > -0.9 || ahead[1] <= 0 {
		test.Fatalf("looking %v after turning left and up", ahead)
	}
}

func TestTheHeadNeverLooksPastStraightUp(test *testing.T) {
	head := StandingHead()
	head.Turn(0, 10)
	if head.Pitch > mostPitch+1e-9 {
		test.Fatalf("pitch %v past %v", head.Pitch, mostPitch)
	}
}

func TestTheEyesStandApartAcrossTheHead(test *testing.T) {
	head := StandingHead()
	head.Turn(math.Pi/2, 0) // looking along -X, the right eye is toward -Z
	eyes := Eyes(head.Pose(), 800, 600)
	apart := eyes[1].Pose.Position[2] - eyes[0].Pose.Position[2]
	if !near(apart, -EyesApartMetres) || !near(eyes[0].Pose.Position[1], EyeHeightMetres) {
		test.Fatalf("eyes at %v and %v", eyes[0].Pose.Position, eyes[1].Pose.Position)
	}
	if eyes[0].Width != 800 || eyes[0].Height != 600 {
		test.Fatalf("eye picture %d × %d", eyes[0].Width, eyes[0].Height)
	}
}

func TestAWidePictureSeesFurtherToTheSidesThanUp(test *testing.T) {
	field := FieldFor(1600, 800)
	if field.RightRadians <= field.UpRadians || !near(field.LeftRadians, -field.RightRadians) {
		test.Fatalf("field %+v", field)
	}
	square := FieldFor(500, 500)
	if !near(square.RightRadians, HalfHeightRadians) {
		test.Fatalf("a square picture sees %v to the side, want %v", square.RightRadians, HalfHeightRadians)
	}
}

func TestTheKeysPushTheSticksAndPressTheButtons(test *testing.T) {
	keys := Keys{Forward: true, Left: true, TurnRight: true, Trigger: true, A: true, Y: true, Menu: true}
	controls := keys.Controls(openxr.Hand{Tracked: true}, openxr.Hand{Tracked: true})
	if controls.Left.StickX != -1 || controls.Left.StickY != 1 || controls.Right.StickX != 1 {
		test.Fatalf("sticks %+v %+v", controls.Left, controls.Right)
	}
	if controls.Right.Trigger != 1 || !controls.Right.Lower || !controls.Left.Upper || !controls.Menu {
		test.Fatalf("buttons %+v", controls)
	}
	state := input.NewState()
	controls.Apply(state)
	if !state.PadDown(input.PadA) || !state.PadDown(input.PadY) || state.PadDown(input.PadB) {
		test.Fatal("the pad does not hold what the keys press")
	}
	if x, y := state.Stick(); x != -1 || y != 1 {
		test.Fatalf("the pad's stick is at %v, %v", x, y)
	}
}

func TestTheWheelAddsToTheRightStickWithinItsReach(test *testing.T) {
	controls := Keys{Up: true, Scroll: 1}.Controls(openxr.Hand{}, openxr.Hand{})
	if controls.Right.StickY != 1 {
		test.Fatalf("stick at %v, want held at 1", controls.Right.StickY)
	}
}

func TestTheRightHandPointsWhereTheCursorIsOnAPanel(test *testing.T) {
	head := StandingHead().Pose()
	panel := openxr.DetachedPanel(1000, 500)
	panel.Pose = graphics.Pose{Orientation: graphics.IdentityQuaternion(), Position: graphics.Vector{0, EyeHeightMetres, -2, 1}}
	panel.FitWidth(2)
	panel.Shown = true
	eyes := Eyes(head, 800, 800)
	hand := RightHand(head, eyes[0], 0.5, 0.5, []*openxr.Panel{panel})
	hit, hits := pointer.HitPanel(pointer.FromAim(hand.Aim), panel)
	if !hits || !hand.Tracked {
		test.Fatal("the hand misses the panel under the cursor")
	}
	// the cursor is in the middle of the left eye's picture: a little left of the panel's middle
	if math.Abs(float64(hit.X-500)) > 20 || math.Abs(float64(hit.Y-250)) > 5 {
		test.Fatalf("the hand points at pixel %v, %v, want about the middle", hit.X, hit.Y)
	}
}

func TestOverNoPanelTheRightHandPointsFarAhead(test *testing.T) {
	head := StandingHead().Pose()
	hand := RightHand(head, Eyes(head, 800, 800)[1], 0.5, 0.5, nil)
	ahead := forward(hand.Aim)
	if ahead[2] > -0.99 {
		test.Fatalf("pointing %v, want almost straight ahead", ahead)
	}
}

func TestTheLeftHandRestsAheadOrTurnsItsPalmUp(test *testing.T) {
	head := StandingHead().Pose()
	resting := LeftHand(head, false)
	if resting.Grip.Position[0] >= 0 || !near(forward(resting.Aim)[2], -1) {
		test.Fatalf("resting left hand at %v pointing %v", resting.Grip.Position, forward(resting.Aim))
	}
	palm := LeftHand(head, true)
	if palm.Grip == resting.Grip {
		test.Fatal("the palm did not turn")
	}
}

func TestAWheelNotchPushesTheStickUntilItIsScrolled(test *testing.T) {
	var wheel Wheel
	wheel.Add(1)
	pushes, total := 0, 0.0
	for push := wheel.Push(0.02); push != 0; push = wheel.Push(0.02) {
		pushes++
		total += float64(push) * pointer.ScrollNotchesPerSecond * 0.02
		if pushes > 100 {
			test.Fatal("the wheel never stops pushing")
		}
	}
	if total < 0.9 || total > 1.3 {
		test.Fatalf("scrolled %v notches for one", total)
	}
	wheel.Add(-2)
	if wheel.Push(0.02) != -1 {
		test.Fatal("turning the wheel down does not push the stick down")
	}
}
