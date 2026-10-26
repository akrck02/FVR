package desktopgl

import "testing"

func TestAnOpenGLESShaderIsReadAsDesktopGLSL(test *testing.T) {
	source := "#version 100\nprecision mediump float;\nuniform vec4 colour;\nvoid main() { gl_FragColor = colour; }\n"
	want := "#version 120\nuniform vec4 colour;\nvoid main() { gl_FragColor = colour; }\n"
	if got := Shader(source); got != want {
		test.Fatalf("Shader gave\n%q\nwant\n%q", got, want)
	}
}

func TestAShaderWithNothingToChangeStaysAsItWas(test *testing.T) {
	source := "#version 120\n// precision is a word in this comment\nvoid main() {}"
	if got := Shader(source); got != source {
		test.Fatalf("Shader changed %q into %q", source, got)
	}
}
