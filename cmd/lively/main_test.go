package main

import "testing"

func BenchmarkGeneration(b *testing.B) {
	openmwcfg := "/home/ern/tes3/config/openmw.cfg"
	for b.Loop() {
		sync(openmwcfg)
	}
}
