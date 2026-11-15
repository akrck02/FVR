// Package desktopgl is go-renderer's gles.Functions on a computer's OpenGL: the 2.1 context every
// desktop still offers (macOS's default one among them), which has all of OpenGL ES 2.0 the
// renderers ask for. So a program written for the headset draws, unchanged, into a window.
//
// The one difference is the shading language: the renderers' shaders are GLSL ES 1.00, and desktop
// OpenGL reads them as GLSL 1.20 once the version line is changed and the precision lines, which it
// does not know, are taken out (Shader does both).
package desktopgl

import (
	"strings"
	"unsafe"

	"github.com/akrck02/go-renderer/gles"
	"github.com/go-gl/gl/v2.1/gl"
)

// Functions calls OpenGL in the context that is current on the calling thread. Call Init once, with
// the context current, before using it.
type Functions struct{}

// Init loads OpenGL's functions for the context current on this thread.
func Init() error { return gl.Init() }

// Shader is an OpenGL ES 2.0 shader's source as desktop OpenGL 2.1 reads it: "#version 100"
// becomes "#version 120" and each "precision …;" line goes (1.20 has no precision qualifiers, and
// gives every float full precision anyway). Any other source comes back as it was.
func Shader(source string) string {
	lines := strings.Split(source, "\n")
	kept := lines[:0]
	for _, line := range lines {
		trimmed := strings.TrimSpace(line)
		switch {
		case trimmed == "#version 100":
			kept = append(kept, "#version 120")
		case strings.HasPrefix(trimmed, "precision ") && strings.HasSuffix(trimmed, ";"):
		default:
			kept = append(kept, line)
		}
	}
	return strings.Join(kept, "\n")
}

var _ gles.Functions = Functions{}

func (Functions) CreateShader(stage gles.Enum) gles.Shader {
	return gles.Shader(gl.CreateShader(uint32(stage)))
}

func (Functions) SetShaderSource(shader gles.Shader, source string) {
	text, free := gl.Strs(Shader(source) + "\x00")
	defer free()
	gl.ShaderSource(uint32(shader), 1, text, nil)
}

func (Functions) CompileShader(shader gles.Shader) { gl.CompileShader(uint32(shader)) }

func (Functions) ShaderCompiled(shader gles.Shader) (bool, string) {
	var status, length int32
	gl.GetShaderiv(uint32(shader), gl.COMPILE_STATUS, &status)
	gl.GetShaderiv(uint32(shader), gl.INFO_LOG_LENGTH, &length)
	return status == gl.TRUE, readLog(length, func(size int32, written *int32, text *uint8) {
		gl.GetShaderInfoLog(uint32(shader), size, written, text)
	})
}

func (Functions) DeleteShader(shader gles.Shader) { gl.DeleteShader(uint32(shader)) }

func (Functions) CreateProgram() gles.Program { return gles.Program(gl.CreateProgram()) }

func (Functions) AttachShader(program gles.Program, shader gles.Shader) {
	gl.AttachShader(uint32(program), uint32(shader))
}

func (Functions) LinkProgram(program gles.Program) { gl.LinkProgram(uint32(program)) }

func (Functions) ProgramLinked(program gles.Program) (bool, string) {
	var status, length int32
	gl.GetProgramiv(uint32(program), gl.LINK_STATUS, &status)
	gl.GetProgramiv(uint32(program), gl.INFO_LOG_LENGTH, &length)
	return status == gl.TRUE, readLog(length, func(size int32, written *int32, text *uint8) {
		gl.GetProgramInfoLog(uint32(program), size, written, text)
	})
}

// readLog reads a compiler's or linker's log of length bytes (the ending zero counted).
func readLog(length int32, read func(size int32, written *int32, text *uint8)) string {
	if length <= 1 {
		return ""
	}
	text := make([]uint8, length)
	var written int32
	read(length, &written, &text[0])
	return string(text[:written])
}

func (Functions) BindAttributeLocation(program gles.Program, index uint32, name string) {
	gl.BindAttribLocation(uint32(program), index, gl.Str(name+"\x00"))
}

func (Functions) UseProgram(program gles.Program) { gl.UseProgram(uint32(program)) }

func (Functions) UniformLocation(program gles.Program, name string) gles.Uniform {
	return gles.Uniform(gl.GetUniformLocation(uint32(program), gl.Str(name+"\x00")))
}

func (Functions) SetUniformInteger(location gles.Uniform, value int32) {
	gl.Uniform1i(int32(location), value)
}

func (Functions) SetUniformFloat(location gles.Uniform, value float32) {
	gl.Uniform1f(int32(location), value)
}

func (Functions) SetUniformTwoFloats(location gles.Uniform, first, second float32) {
	gl.Uniform2f(int32(location), first, second)
}

func (Functions) SetUniformThreeFloats(location gles.Uniform, first, second, third float32) {
	gl.Uniform3f(int32(location), first, second, third)
}

func (Functions) SetUniformFourFloats(location gles.Uniform, first, second, third, fourth float32) {
	gl.Uniform4f(int32(location), first, second, third, fourth)
}

func (Functions) SetUniformMatrix3By3(location gles.Uniform, matrix [9]float32) {
	gl.UniformMatrix3fv(int32(location), 1, false, &matrix[0])
}

func (Functions) SetUniformMatrix4By4(location gles.Uniform, matrix [16]float32) {
	gl.UniformMatrix4fv(int32(location), 1, false, &matrix[0])
}

func (Functions) CreateBuffer() gles.Buffer {
	var buffer uint32
	gl.GenBuffers(1, &buffer)
	return gles.Buffer(buffer)
}

func (Functions) BindBuffer(target gles.Enum, buffer gles.Buffer) {
	gl.BindBuffer(uint32(target), uint32(buffer))
}

func (Functions) SetBufferData(target gles.Enum, data []byte, usage gles.Enum) {
	gl.BufferData(uint32(target), len(data), bytes(data), uint32(usage))
}

func (Functions) EnableVertexAttribute(index uint32) { gl.EnableVertexAttribArray(index) }

func (Functions) DisableVertexAttribute(index uint32) { gl.DisableVertexAttribArray(index) }

func (Functions) DescribeVertexAttribute(index uint32, size int, kind gles.Enum, normalized bool, stride, offset int) {
	gl.VertexAttribPointerWithOffset(index, int32(size), uint32(kind), normalized, int32(stride), uintptr(offset))
}

func (Functions) CreateTexture() gles.Texture {
	var texture uint32
	gl.GenTextures(1, &texture)
	return gles.Texture(texture)
}

func (Functions) SelectTextureUnit(unit gles.Enum) { gl.ActiveTexture(uint32(unit)) }

func (Functions) BindTexture(target gles.Enum, texture gles.Texture) {
	gl.BindTexture(uint32(target), uint32(texture))
}

func (Functions) SetTextureParameter(target, name gles.Enum, value int32) {
	gl.TexParameteri(uint32(target), uint32(name), value)
}

func (Functions) SetPixelStorage(name gles.Enum, value int32) { gl.PixelStorei(uint32(name), value) }

func (Functions) SetTextureImage(target gles.Enum, level int, internalFormat gles.Enum, width, height int, format, kind gles.Enum, pixels []byte) {
	gl.TexImage2D(uint32(target), int32(level), int32(internalFormat), int32(width), int32(height), 0,
		uint32(format), uint32(kind), bytes(pixels))
}

func (Functions) ReplaceTexturePart(target gles.Enum, level, x, y, width, height int, format, kind gles.Enum, pixels []byte) {
	gl.TexSubImage2D(uint32(target), int32(level), int32(x), int32(y), int32(width), int32(height),
		uint32(format), uint32(kind), bytes(pixels))
}

func (Functions) CopyFramebufferToTexture(target gles.Enum, level, xOffset, yOffset, x, y, width, height int) {
	gl.CopyTexSubImage2D(uint32(target), int32(level), int32(xOffset), int32(yOffset), int32(x), int32(y),
		int32(width), int32(height))
}

func (Functions) DeleteTexture(texture gles.Texture) {
	name := uint32(texture)
	gl.DeleteTextures(1, &name)
}

func (Functions) CreateFramebuffer() gles.Framebuffer {
	var framebuffer uint32
	gl.GenFramebuffers(1, &framebuffer)
	return gles.Framebuffer(framebuffer)
}

func (Functions) BindFramebuffer(target gles.Enum, framebuffer gles.Framebuffer) {
	gl.BindFramebuffer(uint32(target), uint32(framebuffer))
}

func (Functions) AttachTextureToFramebuffer(target, attachment, textureTarget gles.Enum, texture gles.Texture, level int) {
	gl.FramebufferTexture2D(uint32(target), uint32(attachment), uint32(textureTarget), uint32(texture), int32(level))
}

func (Functions) DeleteFramebuffer(framebuffer gles.Framebuffer) {
	name := uint32(framebuffer)
	gl.DeleteFramebuffers(1, &name)
}

func (Functions) SetViewport(x, y, width, height int) {
	gl.Viewport(int32(x), int32(y), int32(width), int32(height))
}

func (Functions) SetClearColor(red, green, blue, alpha float32) {
	gl.ClearColor(red, green, blue, alpha)
}

func (Functions) Clear(mask gles.Enum) { gl.Clear(uint32(mask)) }

func (Functions) Enable(capability gles.Enum) { gl.Enable(uint32(capability)) }

func (Functions) Disable(capability gles.Enum) { gl.Disable(uint32(capability)) }

func (Functions) SetBlendFactors(sourceColor, destinationColor, sourceAlpha, destinationAlpha gles.Enum) {
	gl.BlendFuncSeparate(uint32(sourceColor), uint32(destinationColor), uint32(sourceAlpha), uint32(destinationAlpha))
}

func (Functions) SetDepthWrite(write bool) { gl.DepthMask(write) }

func (Functions) SetDepthFunction(function gles.Enum) { gl.DepthFunc(uint32(function)) }

func (Functions) DrawArrays(mode gles.Enum, first, count int) {
	gl.DrawArrays(uint32(mode), int32(first), int32(count))
}

func (Functions) DrawElements(mode gles.Enum, count int, kind gles.Enum, offset int) {
	gl.DrawElementsWithOffset(uint32(mode), int32(count), uint32(kind), uintptr(offset))
}

// bytes points OpenGL at data, or at nothing when there is none (a texture made empty, to be drawn
// into).
func bytes(data []byte) unsafe.Pointer {
	if len(data) == 0 {
		return nil
	}
	return unsafe.Pointer(&data[0])
}
