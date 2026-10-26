package headset

import (
	"errors"
	"fmt"
	"image"
	"image/png"
	"os"

	"github.com/akrck02/go-renderer/graphics"
	"github.com/akrck02/go-renderer/openxr"
	"github.com/go-gl/gl/v2.1/gl"
)

// What a headset's compositor does, done with OpenGL in the window: the eyes' pictures and the
// panels' are framebuffers, the panels are laid over each eye as the room would show them, and the
// eyes are copied to the window.

// target is a picture drawn into: a framebuffer with a colour texture and, for an eye, a depth
// buffer.
type target struct {
	framebuffer, colour, depth uint32
	width, height              int
}

// newTarget makes a picture of width × height pixels, with a depth buffer when depth asks.
func newTarget(width, height int, depth bool) (*target, error) {
	made := &target{width: width, height: height}
	gl.GenTextures(1, &made.colour)
	gl.BindTexture(gl.TEXTURE_2D, made.colour)
	gl.TexParameteri(gl.TEXTURE_2D, gl.TEXTURE_MIN_FILTER, gl.LINEAR)
	gl.TexParameteri(gl.TEXTURE_2D, gl.TEXTURE_MAG_FILTER, gl.LINEAR)
	gl.TexParameteri(gl.TEXTURE_2D, gl.TEXTURE_WRAP_S, gl.CLAMP_TO_EDGE)
	gl.TexParameteri(gl.TEXTURE_2D, gl.TEXTURE_WRAP_T, gl.CLAMP_TO_EDGE)
	gl.TexImage2D(gl.TEXTURE_2D, 0, gl.RGBA8, int32(width), int32(height), 0, gl.RGBA, gl.UNSIGNED_BYTE, nil)
	gl.BindTexture(gl.TEXTURE_2D, 0)
	gl.GenFramebuffers(1, &made.framebuffer)
	gl.BindFramebuffer(gl.FRAMEBUFFER, made.framebuffer)
	gl.FramebufferTexture2D(gl.FRAMEBUFFER, gl.COLOR_ATTACHMENT0, gl.TEXTURE_2D, made.colour, 0)
	if depth {
		gl.GenRenderbuffers(1, &made.depth)
		gl.BindRenderbuffer(gl.RENDERBUFFER, made.depth)
		gl.RenderbufferStorage(gl.RENDERBUFFER, gl.DEPTH_COMPONENT24, int32(width), int32(height))
		gl.FramebufferRenderbuffer(gl.FRAMEBUFFER, gl.DEPTH_ATTACHMENT, gl.RENDERBUFFER, made.depth)
		gl.BindRenderbuffer(gl.RENDERBUFFER, 0)
	}
	status := gl.CheckFramebufferStatus(gl.FRAMEBUFFER)
	gl.ClearColor(0, 0, 0, 0)
	gl.Clear(gl.COLOR_BUFFER_BIT | gl.DEPTH_BUFFER_BIT)
	gl.BindFramebuffer(gl.FRAMEBUFFER, 0)
	if status != gl.FRAMEBUFFER_COMPLETE {
		made.delete()
		return nil, fmt.Errorf("headset: a %d × %d picture cannot be drawn into (framebuffer status %#x)", width, height, status)
	}
	return made, nil
}

func (made *target) delete() {
	gl.DeleteFramebuffers(1, &made.framebuffer)
	gl.DeleteTextures(1, &made.colour)
	if made.depth != 0 {
		gl.DeleteRenderbuffers(1, &made.depth)
	}
}

// composeUnit is the texture unit the panels are read from: one go-renderer's renderers do not use,
// so the textures they keep bound stay bound.
const composeUnit = 7

// composeAttributes is how many vertex attributes the compositor looks after: more than any of
// go-renderer's renderers uses.
const composeAttributes = 8

const composeVertexShader = `#version 120
attribute vec2 corner;
uniform mat4 transform;
varying vec2 coordinate;
void main() {
    coordinate = corner + 0.5;
    gl_Position = transform * vec4(corner, 0.0, 1.0);
}
`

const composeFragmentShader = `#version 120
uniform sampler2D picture;
varying vec2 coordinate;
void main() {
    gl_FragColor = texture2D(picture, coordinate);
}
`

// composer lays panels over an eye's picture: each a rectangle WidthMetres × HeightMetres at its
// pose, facing +Z, its picture's alpha premultiplied, over whatever the eye shows (a headset shows
// its panels in front of the scene).
type composer struct {
	program, corners  uint32
	transform, sample int32
}

func newComposer() (*composer, error) {
	program, err := linkProgram(composeVertexShader, composeFragmentShader)
	if err != nil {
		return nil, err
	}
	made := &composer{program: program}
	made.transform = gl.GetUniformLocation(program, gl.Str("transform\x00"))
	made.sample = gl.GetUniformLocation(program, gl.Str("picture\x00"))
	corners := []float32{-0.5, -0.5, 0.5, -0.5, -0.5, 0.5, 0.5, 0.5}
	gl.GenBuffers(1, &made.corners)
	gl.BindBuffer(gl.ARRAY_BUFFER, made.corners)
	gl.BufferData(gl.ARRAY_BUFFER, len(corners)*4, gl.Ptr(corners), gl.STATIC_DRAW)
	gl.BindBuffer(gl.ARRAY_BUFFER, 0)
	return made, nil
}

// compose draws the shown panels over the eye's picture, each read from its own picture.
func (made *composer) compose(eye openxr.Eye, panels []*openxr.Panel, pictures []*target) {
	gl.BindFramebuffer(gl.FRAMEBUFFER, uint32(eye.Target))
	gl.Viewport(0, 0, int32(eye.Width), int32(eye.Height))
	gl.Disable(gl.DEPTH_TEST)
	gl.DepthMask(false)
	gl.Enable(gl.BLEND)
	gl.BlendFuncSeparate(gl.ONE, gl.ONE_MINUS_SRC_ALPHA, gl.ONE, gl.ONE_MINUS_SRC_ALPHA)
	gl.UseProgram(made.program)
	gl.Uniform1i(made.sample, composeUnit)
	gl.ActiveTexture(gl.TEXTURE0 + composeUnit)
	gl.BindBuffer(gl.ARRAY_BUFFER, made.corners)
	// go-renderer's renderers switch their attributes on once and count on them staying on: switch
	// off for the panels those that would read the renderers' vertices, and back on after.
	var enabled [composeAttributes]int32
	for index := range uint32(composeAttributes) {
		gl.GetVertexAttribiv(index, gl.VERTEX_ATTRIB_ARRAY_ENABLED, &enabled[index])
		gl.DisableVertexAttribArray(index)
	}
	gl.EnableVertexAttribArray(0)
	gl.VertexAttribPointerWithOffset(0, 2, gl.FLOAT, false, 0, 0)
	projection := graphics.PerspectiveFromFieldOfView(eye.Field, 0.05, 1000)
	seen := projection.Multiply(eye.Pose.ViewMatrix())
	for index, panel := range panels {
		if !panel.Shown || panel.WidthMetres <= 0 || panel.HeightMetres <= 0 {
			continue
		}
		size := graphics.Scale(graphics.Vector{panel.WidthMetres, panel.HeightMetres, 1, 1})
		transform := seen.Multiply(panel.Pose.Matrix()).Multiply(size)
		gl.UniformMatrix4fv(made.transform, 1, false, &transform[0])
		gl.BindTexture(gl.TEXTURE_2D, pictures[index].colour)
		gl.DrawArrays(gl.TRIANGLE_STRIP, 0, 4)
	}
	gl.BindTexture(gl.TEXTURE_2D, 0)
	gl.ActiveTexture(gl.TEXTURE0)
	for index, on := range enabled {
		if on == 0 {
			gl.DisableVertexAttribArray(uint32(index))
		} else {
			gl.EnableVertexAttribArray(uint32(index))
		}
	}
	gl.BindBuffer(gl.ARRAY_BUFFER, 0)
	gl.UseProgram(0)
	gl.Disable(gl.BLEND)
	gl.DepthMask(true)
	gl.BindFramebuffer(gl.FRAMEBUFFER, 0)
}

// linkProgram compiles and links a program from GLSL 1.20 sources, the attribute corner at index 0.
func linkProgram(vertexSource, fragmentSource string) (uint32, error) {
	vertex, err := compileShader(gl.VERTEX_SHADER, vertexSource)
	if err != nil {
		return 0, err
	}
	fragment, err := compileShader(gl.FRAGMENT_SHADER, fragmentSource)
	if err != nil {
		return 0, err
	}
	program := gl.CreateProgram()
	gl.AttachShader(program, vertex)
	gl.AttachShader(program, fragment)
	gl.BindAttribLocation(program, 0, gl.Str("corner\x00"))
	gl.LinkProgram(program)
	gl.DeleteShader(vertex)
	gl.DeleteShader(fragment)
	var status int32
	gl.GetProgramiv(program, gl.LINK_STATUS, &status)
	if status != gl.TRUE {
		return 0, errors.New("headset: the panels' program does not link")
	}
	return program, nil
}

func compileShader(stage uint32, source string) (uint32, error) {
	shader := gl.CreateShader(stage)
	text, free := gl.Strs(source + "\x00")
	gl.ShaderSource(shader, 1, text, nil)
	free()
	gl.CompileShader(shader)
	var status int32
	gl.GetShaderiv(shader, gl.COMPILE_STATUS, &status)
	if status != gl.TRUE {
		var length int32
		gl.GetShaderiv(shader, gl.INFO_LOG_LENGTH, &length)
		log := make([]uint8, length+1)
		gl.GetShaderInfoLog(shader, length, nil, &log[0])
		return 0, fmt.Errorf("headset: a shader of the panels' does not compile: %s", log)
	}
	return shader, nil
}

// present copies the eyes to the window, width × height pixels: both side by side, or the left one
// filling it.
func present(eyes [2]*target, width, height int, sideBySide bool) {
	gl.BindFramebuffer(gl.DRAW_FRAMEBUFFER, 0)
	gl.Viewport(0, 0, int32(width), int32(height))
	gl.ClearColor(0, 0, 0, 1)
	gl.Clear(gl.COLOR_BUFFER_BIT)
	shown := 1
	if sideBySide {
		shown = 2
	}
	for index := range shown {
		eye := eyes[index]
		left := int32(index * width / shown)
		gl.BindFramebuffer(gl.READ_FRAMEBUFFER, eye.framebuffer)
		gl.BlitFramebuffer(0, 0, int32(eye.width), int32(eye.height), left, 0, left+int32(eye.width), int32(eye.height),
			gl.COLOR_BUFFER_BIT, gl.NEAREST)
	}
	gl.BindFramebuffer(gl.READ_FRAMEBUFFER, 0)
}

// capture saves what the window shows, width × height pixels, as a PNG.
func capture(path string, width, height int) error {
	pixels := make([]byte, width*height*4)
	gl.BindFramebuffer(gl.READ_FRAMEBUFFER, 0)
	gl.ReadBuffer(gl.BACK)
	gl.PixelStorei(gl.PACK_ALIGNMENT, 1)
	gl.ReadPixels(0, 0, int32(width), int32(height), gl.RGBA, gl.UNSIGNED_BYTE, gl.Ptr(pixels))
	picture := image.NewNRGBA(image.Rect(0, 0, width, height))
	for row := range height { // OpenGL's rows go up from the bottom, a PNG's down from the top
		copy(picture.Pix[row*picture.Stride:], pixels[(height-1-row)*width*4:(height-row)*width*4])
	}
	for index := 3; index < len(picture.Pix); index += 4 {
		picture.Pix[index] = 255 // the window is opaque, whatever alpha its pixels keep
	}
	file, err := os.Create(path)
	if err != nil {
		return err
	}
	if err := png.Encode(file, picture); err != nil {
		file.Close()
		return err
	}
	return file.Close()
}
