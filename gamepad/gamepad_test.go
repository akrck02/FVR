package gamepad

import (
	"encoding/binary"
	"math"
	"testing"

	"github.com/akrck02/go-renderer/graphics"
)

func TestASensorEventIsReadFromSDLsBytes(test *testing.T) {
	var bytes [eventSize]byte
	write := binary.LittleEndian
	write.PutUint32(bytes[0:], eventSensorUpdate)
	write.PutUint32(bytes[16:], 7)
	write.PutUint32(bytes[20:], sensorGyroscope)
	for index, value := range []float32{0.5, -1, 2} {
		write.PutUint32(bytes[24+4*index:], math.Float32bits(value))
	}
	write.PutUint64(bytes[40:], 123456789)
	got := readEvent(&bytes)
	want := event{kind: eventSensorUpdate, which: 7, sensor: sensorGyroscope, data: [3]float32{0.5, -1, 2}, timestamp: 123456789}
	if got != want {
		test.Fatalf("read %+v, want %+v", got, want)
	}
}

func TestReadingsTurnAPadOnlyOnceItKnowsWhereUpIs(test *testing.T) {
	reader := &Reader{}
	reader.hands[RightHand] = &pad{handle: 1, id: 3}
	turn := event{kind: eventSensorUpdate, which: 3, sensor: sensorGyroscope, data: [3]float32{0, 1, 0}, timestamp: 1e9}
	reader.handle(turn) // no accelerometer reading yet
	if reader.hands[RightHand].lastTurn != 0 {
		test.Fatal("turned before any accelerometer reading")
	}
	reader.handle(event{kind: eventSensorUpdate, which: 3, sensor: sensorAccelerometer, data: [3]float32{0, 9.8, 0}})
	for step := uint64(0); step <= 100; step++ {
		turn.timestamp = 2e9 + step*5e6 // 200 readings a second, half a second
		reader.handle(turn)
	}
	ahead := reader.hands[RightHand].orientation.Rotation().Rotate(graphics.Vector{0, 0, -1, 0})
	if math.Abs(float64(ahead[0])+math.Sin(0.5)) > 0.02 {
		test.Fatalf("pointing %v, want half a radian to the left", ahead)
	}
}

func TestEachPadTurnsOnlyByItsOwnReadings(test *testing.T) {
	reader := &Reader{}
	reader.hands[LeftHand], reader.hands[RightHand] = &pad{handle: 1, id: 3}, &pad{handle: 2, id: 4}
	reader.handle(event{kind: eventSensorUpdate, which: 4, sensor: sensorAccelerometer, data: [3]float32{0, 9.8, 0}})
	if reader.hands[LeftHand].pull != [3]float32{} || reader.hands[RightHand].pull == [3]float32{} {
		test.Fatal("a reading went to the wrong pad")
	}
}

func TestJoyConGoToTheirOwnHandsAndAWholePadToTheRightFirst(test *testing.T) {
	reader := &Reader{}
	if hand, free := reader.handFor(Whole); hand != RightHand || !free {
		test.Fatalf("a whole pad goes to hand %d", hand)
	}
	if hand, _ := reader.handFor(LeftJoyCon); hand != LeftHand {
		test.Fatalf("a left Joy-Con goes to hand %d", hand)
	}
	reader.hands[RightHand] = &pad{}
	if hand, free := reader.handFor(Whole); hand != LeftHand || !free {
		test.Fatalf("with the right hand taken a whole pad goes to hand %d", hand)
	}
	if _, free := reader.handFor(RightJoyCon); free {
		test.Fatal("a right Joy-Con took a taken hand")
	}
}

func TestEachKindOfPadHasItsOwnRecenterAndMenuButtons(test *testing.T) {
	for _, each := range []struct {
		kind           Kind
		recenter, menu int
	}{{LeftJoyCon, Capture, Back}, {RightJoyCon, Guide, Start}, {Whole, Back, Start}} {
		state := State{Kind: each.kind}
		if state.RecenterButton() != each.recenter || state.MenuButton() != each.menu {
			test.Fatalf("kind %d: recenter %d menu %d", each.kind, state.RecenterButton(), state.MenuButton())
		}
	}
}
