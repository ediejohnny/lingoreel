package matroska

// Lacing packs several frames into one block. The two lacing bits of the
// block flags select the method.
const (
	lacingMask  = 0x06
	lacingShift = 1
	lacingNone  = 0
	lacingXiph  = 1
	lacingFixed = 2
	lacingEBML  = 3
	xiphRun     = 255
)

// splitLaces returns the frames packed in a block payload, or false when the
// lace sizes are inconsistent with the payload.
func splitLaces(flags byte, payload []byte) ([][]byte, bool) {
	lacing := (flags & lacingMask) >> lacingShift
	if lacing == lacingNone {
		return [][]byte{payload}, true
	}
	if len(payload) == 0 {
		return nil, false
	}
	count := int(payload[0]) + 1
	data := payload[1:]

	var sizes []int
	var ok bool
	switch lacing {
	case lacingXiph:
		sizes, data, ok = xiphLaceSizes(count, data)
	case lacingEBML:
		sizes, data, ok = ebmlLaceSizes(count, data)
	case lacingFixed:
		if len(data)%count != 0 {
			return nil, false
		}
		sizes, ok = make([]int, count-1), true
		for i := range sizes {
			sizes[i] = len(data) / count
		}
	}
	if !ok {
		return nil, false
	}

	frames := make([][]byte, 0, count)
	for _, size := range sizes {
		if size < 0 || size > len(data) {
			return nil, false
		}
		frames = append(frames, data[:size])
		data = data[size:]
	}
	// The last frame takes whatever remains.
	return append(frames, data), true
}

// xiphLaceSizes reads count-1 sizes, each a run of 255 bytes plus a final byte.
func xiphLaceSizes(count int, data []byte) ([]int, []byte, bool) {
	sizes := make([]int, count-1)
	for i := range sizes {
		for {
			if len(data) == 0 {
				return nil, nil, false
			}
			next := data[0]
			data = data[1:]
			sizes[i] += int(next)
			if next != xiphRun {
				break
			}
		}
	}
	return sizes, data, true
}

// ebmlLaceSizes reads the first size as a variable size integer and each
// following size as a signed difference from the previous one.
func ebmlLaceSizes(count int, data []byte) ([]int, []byte, bool) {
	sizes := make([]int, count-1)
	for i := range sizes {
		raw, length, _ := parseVint(data)
		if length == 0 {
			return nil, nil, false
		}
		data = data[length:]
		if i == 0 {
			sizes[i] = int(raw)
			continue
		}
		// Signed values are stored with a bias of half the range.
		bias := int64(1)<<(7*length-1) - 1
		sizes[i] = sizes[i-1] + int(int64(raw)-bias)
	}
	return sizes, data, true
}
