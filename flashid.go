package gc

// Shamelessly lifted from GCMM

const (
	serialLength = 12

	multiply = 1103515245 // 0x41C64E6D
	addition = 12345      // 0x3039

	shiftWidth = 16

	mask = 0x7fff
)

func extractFlashID(serial [serialLength]byte, rand uint64) [serialLength]byte {
	var flashID [serialLength]byte

	for i := 0; i < serialLength; i++ {
		rand = (rand*multiply + addition) >> shiftWidth
		flashID[i] = serial[i] - byte(rand) // subtraction
		rand = (rand*multiply + addition) >> shiftWidth
		rand &= mask
	}

	return flashID
}

func computeSerial(flashID [serialLength]byte, rand uint64) [serialLength]byte {
	var serial [serialLength]byte

	for i := 0; i < serialLength; i++ {
		rand = (rand*multiply + addition) >> shiftWidth
		serial[i] = flashID[i] + byte(rand) // addition
		rand = (rand*multiply + addition) >> shiftWidth
		rand &= mask
	}

	return serial
}
