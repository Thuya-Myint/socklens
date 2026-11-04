package platform

import (
	"encoding/binary"
	"fmt"
	"unicode/utf16"
)

// Decode absolute string pointers only after checking they refer to this buffer.
// ENUM_SERVICE_STATUS_PROCESSW is 56 bytes on 64-bit Windows, 44 on 32-bit.
func parseWindowsServices(data []byte, base uintptr, count uint32, pointerSize int) ([]Service, error) {
	stride := 56
	if pointerSize == 4 {
		stride = 44
	} else if pointerSize != 8 {
		return nil, fmt.Errorf("unsupported service pointer size")
	}
	if uint64(count)*uint64(stride) > uint64(len(data)) {
		return nil, fmt.Errorf("truncated Windows service table")
	}
	pointer := func(offset int) uint64 {
		if pointerSize == 4 {
			return uint64(binary.LittleEndian.Uint32(data[offset:]))
		}
		return binary.LittleEndian.Uint64(data[offset:])
	}
	stringAt := func(address uint64) (string, error) {
		if address < uint64(base) || address-uint64(base) >= uint64(len(data)) || address-uint64(base) < uint64(count)*uint64(stride) || (address-uint64(base))%2 != 0 {
			return "", fmt.Errorf("invalid Windows service string pointer")
		}
		offset := int(address - uint64(base))
		units := []uint16{}
		for offset+2 <= len(data) {
			unit := binary.LittleEndian.Uint16(data[offset:])
			offset += 2
			if unit == 0 {
				return string(utf16.Decode(units)), nil
			}
			units = append(units, unit)
		}
		return "", fmt.Errorf("unterminated Windows service string")
	}
	result := []Service{}
	for i := 0; i < int(count); i++ {
		offset := i * stride
		status := offset + 2*pointerSize
		pid := binary.LittleEndian.Uint32(data[status+28:])
		if pid == 0 {
			continue
		}
		name, err := stringAt(pointer(offset))
		if err != nil {
			return nil, err
		}
		display, err := stringAt(pointer(offset + pointerSize))
		if err != nil {
			return nil, err
		}
		if name == "" {
			return nil, fmt.Errorf("Windows service name is empty")
		}
		kind := "unknown"
		serviceType := binary.LittleEndian.Uint32(data[status:])
		if serviceType&0x20 != 0 {
			kind = "shared-host"
		} else if serviceType&0x10 != 0 {
			kind = "own-process"
		}
		state := binary.LittleEndian.Uint32(data[status+4:])
		label := fmt.Sprintf("unknown(%d)", state)
		if state >= 1 && state <= 7 {
			label = []string{"stopped", "start-pending", "stop-pending", "running", "continue-pending", "pause-pending", "paused"}[state-1]
		}
		result = append(result, Service{PID: int(pid), Name: name, DisplayName: display, State: label, ProcessType: kind, Source: "windows-scm"})
	}
	return result, nil
}
