module github.com/akrck02/FVR

go 1.25.6

require (
	github.com/akrck02/go-meta-quest v0.0.0-00010101000000-000000000000
	github.com/akrck02/go-renderer v0.2.1-0.20261026131534-d02539a78e74
	github.com/go-gl/gl v0.0.0-20260331235117-4566fea9a276
	github.com/go-gl/glfw/v3.4/glfw v0.1.0-pre.1
)

require (
	golang.org/x/image v0.45.0 // indirect
	golang.org/x/sys v0.47.0 // indirect
	golang.org/x/text v0.41.0 // indirect
)

// The headset pieces are worked on side by side with this playground.
replace (
	github.com/akrck02/go-meta-quest => ../go-meta-quest
	github.com/akrck02/go-renderer => ../go-renderer
)
