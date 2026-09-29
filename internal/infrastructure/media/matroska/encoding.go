package matroska

import (
	"bytes"
	"compress/bzip2"
	"compress/zlib"
	"fmt"
	"io"
	"sort"
)

// maxDecodedFrameBytes bounds a decompressed subtitle frame, so a small
// compressed frame cannot expand into gigabytes.
const maxDecodedFrameBytes = 1 << 20

// frameDecoder undoes the content encodings of one track.
type frameDecoder struct {
	layers []ContentEncoding
}

// newFrameDecoder validates a track's encodings. Encryption and LZO are
// reported as unsupported.
func newFrameDecoder(encodings []ContentEncoding) (frameDecoder, error) {
	var layers []ContentEncoding
	for _, encoding := range encodings {
		if encoding.Scope&encodingScopeFrames == 0 {
			continue
		}
		if encoding.Type != encodingTypeCompress {
			return frameDecoder{}, fmt.Errorf("%w: encrypted track", ErrUnsupportedEncoding)
		}
		switch encoding.CompressionAlgo {
		case compressionZlib, compressionBzlib, compressionHeaderStrip:
		case compressionLZO:
			return frameDecoder{}, fmt.Errorf("%w: LZO compression", ErrUnsupportedEncoding)
		default:
			return frameDecoder{}, fmt.Errorf("%w: compression %d", ErrUnsupportedEncoding, encoding.CompressionAlgo)
		}
		layers = append(layers, encoding)
	}
	// Decoding undoes the layers from the highest order to the lowest.
	sort.SliceStable(layers, func(i, j int) bool { return layers[i].Order > layers[j].Order })
	return frameDecoder{layers: layers}, nil
}

func (d frameDecoder) decode(frame []byte) ([]byte, error) {
	for _, layer := range d.layers {
		var err error
		switch layer.CompressionAlgo {
		case compressionHeaderStrip:
			frame = append(append([]byte{}, layer.CompressionSettings...), frame...)
		case compressionZlib:
			frame, err = inflate(func(r io.Reader) (io.Reader, error) { return zlib.NewReader(r) }, frame)
		case compressionBzlib:
			frame, err = inflate(func(r io.Reader) (io.Reader, error) { return bzip2.NewReader(r), nil }, frame)
		}
		if err != nil {
			return nil, err
		}
		if len(frame) > maxDecodedFrameBytes {
			return nil, fmt.Errorf("%w: decoded frame larger than %d bytes", ErrTooLarge, maxDecodedFrameBytes)
		}
	}
	return frame, nil
}

func inflate(open func(io.Reader) (io.Reader, error), compressed []byte) ([]byte, error) {
	reader, err := open(bytes.NewReader(compressed))
	if err != nil {
		return nil, fmt.Errorf("%w: %v", ErrCorrupt, err)
	}
	if closer, ok := reader.(io.Closer); ok {
		defer closer.Close()
	}
	decoded, err := io.ReadAll(io.LimitReader(reader, maxDecodedFrameBytes+1))
	if err != nil {
		return nil, fmt.Errorf("%w: %v", ErrCorrupt, err)
	}
	if len(decoded) > maxDecodedFrameBytes {
		return nil, fmt.Errorf("%w: decoded frame larger than %d bytes", ErrTooLarge, maxDecodedFrameBytes)
	}
	return decoded, nil
}
