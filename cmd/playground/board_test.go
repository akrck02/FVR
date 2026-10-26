package main

import "testing"

func TestATileIsFoundWhereItIsDrawn(test *testing.T) {
	for index := range tileColumns * tileRows {
		area := tileArea(index)
		if found := tileAt(area.X+area.Width/2, area.Y+area.Height/2); found != index {
			test.Fatalf("the middle of tile %d is tile %d", index, found)
		}
		if area.X+area.Width > boardWidth || area.Y+area.Height > boardHeight {
			test.Fatalf("tile %d, %+v, runs off the board", index, area)
		}
	}
	if found := tileAt(5, 5); found != -1 {
		test.Fatalf("the board's corner is tile %d, want none", found)
	}
}

func TestAClickLightsTheHoveredTileAndAnotherPutsItOut(test *testing.T) {
	board := newBoard()
	area := tileArea(4)
	if !board.hover(area.X+10, area.Y+10, true) || board.hovered != 4 {
		test.Fatalf("hovering tile 4 hovers %d", board.hovered)
	}
	if board.hover(area.X+20, area.Y+20, true) {
		test.Fatal("moving within a tile changed the picture")
	}
	if !board.click() || !board.lit[4] {
		test.Fatal("the click did not light tile 4")
	}
	board.click()
	if board.lit[4] {
		test.Fatal("a second click left tile 4 lit")
	}
	board.hover(0, 0, false)
	if board.click() {
		test.Fatal("a click off the board lit a tile")
	}
}

func TestTheBoardIsDrawnAgainInTheSameList(test *testing.T) {
	board := newBoard()
	first := board.draw()
	if first.IsEmpty() {
		test.Fatal("the board draws nothing")
	}
	if second := board.draw(); second != first {
		test.Fatal("the board made a new list")
	}
}
