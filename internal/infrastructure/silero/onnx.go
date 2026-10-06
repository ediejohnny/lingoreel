package silero

import (
	"encoding/binary"
	"errors"
	"fmt"
	"math"
)

// The few parts of the ONNX protobuf format the model's weights need: the
// model's graph, the graph's initializers, and in each tensor its name,
// shape, type and data. Everything else is skipped.
const (
	modelGraph       = 7
	graphInitializer = 5
	tensorDims       = 1
	tensorDataType   = 2
	tensorFloatData  = 4
	tensorName       = 8
	tensorRawData    = 9
	tensorLocation   = 14
	// onnxFloat is TensorProto.FLOAT.
	onnxFloat = 1
)

// Limits on what the reader accepts, well above the model's own (15 tensors,
// the largest of 66,048 values).
const (
	maxTensors  = 64
	maxElements = 1 << 20
)

// errMalformed reports protobuf data the reader cannot follow.
var errMalformed = errors.New("malformed model")

// tensor is a float32 weight and its shape.
type tensor struct {
	shape []int
	data  []float32
}

// readInitializers returns the float32 tensors of a model's main graph by
// name. It never trusts a length it has not checked against the data.
func readInitializers(model []byte) (map[string]tensor, error) {
	graph, err := field(model, modelGraph)
	if err != nil {
		return nil, err
	}
	tensors := make(map[string]tensor)
	err = eachField(graph, func(number int, value []byte, _ uint64) error {
		if number != graphInitializer {
			return nil
		}
		if len(tensors) == maxTensors {
			return fmt.Errorf("%w: more than %d tensors", errMalformed, maxTensors)
		}
		name, t, err := readTensor(value)
		if err != nil {
			return err
		}
		if _, taken := tensors[name]; taken {
			return fmt.Errorf("%w: tensor %q twice", errMalformed, name)
		}
		tensors[name] = t
		return nil
	})
	return tensors, err
}

// readTensor decodes one TensorProto holding float32 data, raw or packed.
func readTensor(data []byte) (string, tensor, error) {
	var name string
	var t tensor
	var raw []byte
	var floats []float32
	dataType := uint64(0)
	err := eachField(data, func(number int, value []byte, scalar uint64) error {
		switch number {
		case tensorDims:
			if value == nil {
				t.shape = append(t.shape, int(scalar))
				return nil
			}
			return eachVarint(value, func(v uint64) { t.shape = append(t.shape, int(v)) })
		case tensorDataType:
			dataType = scalar
		case tensorName:
			name = string(value)
		case tensorRawData:
			raw = value
		case tensorFloatData:
			if value == nil { // one value, not packed
				floats = append(floats, math.Float32frombits(uint32(scalar)))
				return nil
			}
			if len(value)%4 != 0 {
				return fmt.Errorf("%w: float data of %d bytes", errMalformed, len(value))
			}
			for i := 0; i < len(value); i += 4 {
				floats = append(floats, math.Float32frombits(binary.LittleEndian.Uint32(value[i:])))
			}
		case tensorLocation:
			if scalar != 0 {
				return fmt.Errorf("%w: tensor data outside the file", errMalformed)
			}
		}
		return nil
	})
	if err != nil {
		return "", tensor{}, err
	}
	if dataType != onnxFloat {
		return "", tensor{}, fmt.Errorf("%w: tensor %q of type %d, want float", errMalformed, name, dataType)
	}
	elements := 1
	for _, d := range t.shape {
		if d <= 0 || elements > maxElements/d {
			return "", tensor{}, fmt.Errorf("%w: tensor %q of shape %v", errMalformed, name, t.shape)
		}
		elements *= d
	}
	switch {
	case raw != nil && len(raw) == 4*elements:
		t.data = make([]float32, elements)
		for i := range t.data {
			t.data[i] = math.Float32frombits(binary.LittleEndian.Uint32(raw[4*i:]))
		}
	case raw == nil && len(floats) == elements:
		t.data = floats
	default:
		return "", tensor{}, fmt.Errorf("%w: tensor %q holds %d bytes for %d values", errMalformed, name, len(raw)+4*len(floats), elements)
	}
	return name, t, nil
}

// field returns the first length-delimited field of a number.
func field(data []byte, want int) ([]byte, error) {
	var found []byte
	err := eachField(data, func(number int, value []byte, _ uint64) error {
		if number == want && found == nil && value != nil {
			found = value
		}
		return nil
	})
	if err == nil && found == nil {
		err = fmt.Errorf("%w: field %d missing", errMalformed, want)
	}
	return found, err
}

// eachField walks a protobuf message: a length-delimited field comes as
// value, a number as scalar (value nil).
func eachField(data []byte, visit func(number int, value []byte, scalar uint64) error) error {
	for len(data) > 0 {
		key, n := binary.Uvarint(data)
		if n <= 0 || key>>3 == 0 || key>>3 > math.MaxInt32 {
			return fmt.Errorf("%w: bad field key", errMalformed)
		}
		data = data[n:]
		number := int(key >> 3)
		var err error
		switch key & 7 {
		case 0: // varint
			v, n := binary.Uvarint(data)
			if n <= 0 {
				return fmt.Errorf("%w: bad varint", errMalformed)
			}
			data = data[n:]
			err = visit(number, nil, v)
		case 1: // fixed 64 bits
			if len(data) < 8 {
				return fmt.Errorf("%w: short fixed64", errMalformed)
			}
			err = visit(number, nil, binary.LittleEndian.Uint64(data))
			data = data[8:]
		case 2: // length-delimited
			length, n := binary.Uvarint(data)
			if n <= 0 || length > uint64(len(data)-n) {
				return fmt.Errorf("%w: length past the data", errMalformed)
			}
			data = data[n:]
			err = visit(number, data[:length:length], 0)
			data = data[length:]
		case 5: // fixed 32 bits
			if len(data) < 4 {
				return fmt.Errorf("%w: short fixed32", errMalformed)
			}
			err = visit(number, nil, uint64(binary.LittleEndian.Uint32(data)))
			data = data[4:]
		default:
			return fmt.Errorf("%w: wire type %d", errMalformed, key&7)
		}
		if err != nil {
			return err
		}
	}
	return nil
}

// eachVarint walks a packed run of varints.
func eachVarint(data []byte, visit func(uint64)) error {
	for len(data) > 0 {
		v, n := binary.Uvarint(data)
		if n <= 0 {
			return fmt.Errorf("%w: bad packed varint", errMalformed)
		}
		visit(v)
		data = data[n:]
	}
	return nil
}
