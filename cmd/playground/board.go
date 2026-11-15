package main

import (
	"github.com/akrck02/go-renderer/draw2d"
)

// The board's picture, in pixels: about a metre wide at arm's length reads as a laptop's screen.
const (
	boardWidth  = 1024
	boardHeight = 640
)

// boardMetres is how wide the board stands in the room.
const boardMetres = 1.1

// The tiles' grid on the board.
const (
	tileColumns = 3
	tileRows    = 2
	tileMargin  = 48
	tileGap     = 32
	titleHeight = 96
)

var (
	boardColour   = draw2d.MustHex("#f4f1ea")
	titleColour   = draw2d.MustHex("#1d2433")
	tileColour    = draw2d.MustHex("#ffffff")
	litColour     = draw2d.MustHex("#ffcf4a")
	hoverBorder   = draw2d.MustHex("#3b6fd9")
	restingBorder = draw2d.MustHex("#d5d0c4")
)

// board is a few tiles the laser lights: each click turns one on or off.
type board struct {
	lit     [tileColumns * tileRows]bool
	hovered int // the tile under the cursor, -1 for none
	list    draw2d.List
}

func newBoard() *board { return &board{hovered: -1} }

// tileArea is where a tile is on the board.
func tileArea(index int) draw2d.Rectangle {
	width := float32(boardWidth-2*tileMargin-(tileColumns-1)*tileGap) / tileColumns
	height := float32(boardHeight-titleHeight-tileMargin-(tileRows-1)*tileGap) / tileRows
	column, row := index%tileColumns, index/tileColumns
	return draw2d.Rectangle{X: tileMargin + float32(column)*(width+tileGap), Y: titleHeight + float32(row)*(height+tileGap),
		Width: width, Height: height}
}

// tileAt is the tile at a pixel of the board, -1 for none.
func tileAt(x, y float32) int {
	for index := range tileColumns * tileRows {
		area := tileArea(index)
		if x >= area.X && x < area.X+area.Width && y >= area.Y && y < area.Y+area.Height {
			return index
		}
	}
	return -1
}

// hover moves the cursor (off the board: pointing false) and says whether the picture changed.
func (board *board) hover(x, y float32, pointing bool) bool {
	hovered := -1
	if pointing {
		hovered = tileAt(x, y)
	}
	changed := hovered != board.hovered
	board.hovered = hovered
	return changed
}

// click turns the hovered tile on or off, and says whether there was one.
func (board *board) click() bool {
	if board.hovered < 0 {
		return false
	}
	board.lit[board.hovered] = !board.lit[board.hovered]
	return true
}

// draw is the board's picture, made again in the same list.
func (board *board) draw() *draw2d.List {
	board.list.Reset()
	board.list.Text("Point and pull the trigger", tileMargin, 64, 36, draw2d.Bold, titleColour)
	for index, lit := range board.lit {
		fill, border := tileColour, restingBorder
		if lit {
			fill = litColour
		}
		if index == board.hovered {
			border = hoverBorder
		}
		board.list.Rectangle(tileArea(index), 24, fill, border, 6)
	}
	return &board.list
}
