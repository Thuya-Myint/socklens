package platform

import (
	"encoding/binary"
	"testing"
	"unicode/utf16"
)

func serviceFixture(pointerSize int) []byte {
	stride := 56
	if pointerSize == 4 {
		stride = 44
	}
	data := make([]byte, stride+512)
	cursor := stride
	putString := func(value string) uint64 {
		address := uint64(0x1000 + cursor)
		for _, unit := range append(utf16.Encode([]rune(value)), 0) {
			binary.LittleEndian.PutUint16(data[cursor:], unit)
			cursor += 2
		}
		return address
	}
	name := putString("Apple Mobile Device Service")
	display := putString("Apple Mobile Device Service — test")
	if pointerSize == 4 {
		binary.LittleEndian.PutUint32(data, uint32(name))
		binary.LittleEndian.PutUint32(data[4:], uint32(display))
	} else {
		binary.LittleEndian.PutUint64(data, name)
		binary.LittleEndian.PutUint64(data[8:], display)
	}
	status := 2 * pointerSize
	binary.LittleEndian.PutUint32(data[status:], 0x20)
	binary.LittleEndian.PutUint32(data[status+4:], 4)
	binary.LittleEndian.PutUint32(data[status+28:], 8100)
	return data
}
func TestWindowsServiceTableDecoder(t *testing.T) {
	for _, pointerSize := range []int{4, 8} {
		data := serviceFixture(pointerSize)
		services, err := parseWindowsServices(data, 0x1000, 1, pointerSize)
		if err != nil || len(services) != 1 {
			t.Fatal(services, err)
		}
		got := services[0]
		if got.PID != 8100 || got.Name != "Apple Mobile Device Service" || got.DisplayName != "Apple Mobile Device Service — test" || got.State != "running" || got.ProcessType != "shared-host" || got.Source != "windows-scm" {
			t.Fatal(got)
		}
		data = serviceFixture(pointerSize)
		binary.LittleEndian.PutUint32(data[2*pointerSize+28:], 0)
		if services, err := parseWindowsServices(data, 0x1000, 1, pointerSize); err != nil || len(services) != 0 {
			t.Fatal("stopped/unassociated PID retained", services, err)
		}
	}
	for _, tc := range []string{"truncated", "pointer-before-buffer", "pointer-in-records", "odd-pointer", "unterminated"} {
		t.Run(tc, func(t *testing.T) {
			data := serviceFixture(8)
			count := uint32(1)
			switch tc {
			case "truncated":
				count = 100
			case "pointer-before-buffer":
				binary.LittleEndian.PutUint64(data, 0xfff)
			case "pointer-in-records":
				binary.LittleEndian.PutUint64(data, 0x1000)
			case "odd-pointer":
				binary.LittleEndian.PutUint64(data, 0x1039)
			case "unterminated":
				binary.LittleEndian.PutUint64(data, uint64(0x1000+len(data)-2))
				binary.LittleEndian.PutUint16(data[len(data)-2:], 1)
			}
			if _, err := parseWindowsServices(data, 0x1000, count, 8); err == nil {
				t.Fatal("malformed service data accepted")
			}
		})
	}
}
func TestMissingNameFallbackPreservesPreferredNames(t *testing.T) {
	rows := []Socket{{Processes: []Process{{PID: 8100}, {PID: 20, Name: "preferred.exe"}, {PID: 30}}}}
	fillMissingProcessNames(rows, map[int]string{8100: "AppleMobileDeviceService.exe", 20: "fallback.exe"})
	if rows[0].Processes[0].Name != "AppleMobileDeviceService.exe" || rows[0].Processes[1].Name != "preferred.exe" || rows[0].Processes[2].Name != "" {
		t.Fatal(rows)
	}
}
