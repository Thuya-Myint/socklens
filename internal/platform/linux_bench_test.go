//go:build linux

package platform

import "testing"

func BenchmarkListSockets(b *testing.B) {
	b.ReportAllocs()
	for i := 0; i < b.N; i++ {
		if _, err := List(); err != nil {
			b.Fatal(err)
		}
	}
}
