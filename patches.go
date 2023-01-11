package gc

import (
	"bytes"
	"fmt"
	"io"
)

const (
	lengthFZero = 0x8000
	lengthPSO   = 0x6000
)

//nolint:gomnd
func patchFZero(r io.Reader, mc *memoryCard) (io.Reader, error) {
	serial1, serial2 := mc.serialNumbers()

	b, err := io.ReadAll(r)
	if err != nil {
		return nil, fmt.Errorf("unable to read save data: %w", err)
	}

	if len(b) < lengthFZero {
		return nil, errInvalidLength
	}

	b[0x2066] = byte(serial1 >> 24)
	b[0x2067] = byte(serial1 >> 16)
	b[0x7580] = byte(serial2 >> 24)
	b[0x7581] = byte(serial2 >> 16)
	b[0x2060] = byte(serial1 >> 8)
	b[0x2061] = byte(serial1)
	b[0x2200] = byte(serial2 >> 8)
	b[0x2201] = byte(serial2)

	var chksum uint16 = 0xffff

	for i := 2; i < 0x8000; i++ {
		chksum ^= uint16(b[i])
		for j := 8; j > 0; j-- {
			if chksum&1 == 1 {
				chksum = (chksum >> 1) ^ 0x8408
			} else {
				chksum >>= 1
			}
		}
	}

	b[0x0000] = byte(^chksum >> 8)
	b[0x0001] = byte(^chksum)

	return bytes.NewReader(b), nil
}

const (
	offsetPSO12 = 0x00
	offsetPSO3  = 0x10
)

func patchPSO12(r io.Reader, mc *memoryCard) (io.Reader, error) {
	return patchPSO(r, mc, offsetPSO12)
}

func patchPSO3(r io.Reader, mc *memoryCard) (io.Reader, error) {
	return patchPSO(r, mc, offsetPSO3)
}

//nolint:gomnd
func patchPSO(r io.Reader, mc *memoryCard, offset int) (io.Reader, error) {
	serial1, serial2 := mc.serialNumbers()

	b, err := io.ReadAll(r)
	if err != nil {
		return nil, fmt.Errorf("unable to read save data: %w", err)
	}

	if len(b) < lengthPSO {
		return nil, errInvalidLength
	}

	b[0x2158] = byte(serial1 >> 24)
	b[0x2159] = byte(serial1 >> 16)
	b[0x215a] = byte(serial1 >> 8)
	b[0x215b] = byte(serial1)
	b[0x215c] = byte(serial2 >> 24)
	b[0x215d] = byte(serial2 >> 16)
	b[0x215e] = byte(serial2 >> 8)
	b[0x215f] = byte(serial2)

	lut := make([]uint32, 256)

	for i := 0; i < 256; i++ {
		chksum := uint32(i)
		for j := 8; j > 0; j-- {
			if chksum&1 == 1 {
				chksum = (chksum >> 1) ^ 0xedb88320
			} else {
				chksum >>= 1
			}
		}

		lut[i] = chksum
	}

	var chksum uint32 = 0xdebb20e3

	for i := 0x204c; i < 0x2164+offset; i++ {
		chksum = (chksum>>8)&0xffffff ^ lut[byte(chksum)^b[i]]
	}

	chksum ^= 0xffffffff

	b[0x2048] = byte(chksum >> 24)
	b[0x2049] = byte(chksum >> 16)
	b[0x204a] = byte(chksum >> 8)
	b[0x204b] = byte(chksum)

	return bytes.NewReader(b), nil
}
