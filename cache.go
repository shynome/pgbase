package pgbase

import "github.com/maypok86/otter/v2"

var Cache = otter.Must(&otter.Options[string, string]{
	MaximumSize: 1000,
})
