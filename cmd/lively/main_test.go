package main

import "testing"

func ref[T any](v T) *T {
	return &v
}

func BenchmarkGeneration(b *testing.B) {
	openmwCfgPath = ref("/home/ern/tes3/config/openmw.cfg")
	threads = ref(6)
	saveFiles = ref(true)
	mapTextures = ref(true)
	for b.Loop() {
		sync(b.Context())
	}
}
