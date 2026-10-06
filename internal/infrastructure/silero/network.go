package silero

import (
	"fmt"
	"math"

	"github.com/ediejohnny/lingoreel/internal/domain/speech"
)

// The network's geometry at 16 kHz: each chunk is judged with the last
// samples of the one before as context; a short-time Fourier transform of
// that, done as a convolution, feeds four convolutions and a recurrent cell
// whose state goes on to the next chunk.
const (
	contextSamples = 64
	inputSamples   = contextSamples + speech.ChunkSamples // 576
	stftWindow     = 256
	stftHop        = 128
	// stftPad is mirrored onto the end of the input, as the model does.
	stftPad    = 64
	stftBins   = 129
	stftFrames = (inputSamples+stftPad-stftWindow)/stftHop + 1 // 4
	hidden     = 128
)

// The encoder's convolutions: kernel 3, padding 1, channels and strides.
var (
	encoderChannels = [5]int{stftBins, 128, 64, 64, 128}
	encoderStrides  = [4]int{1, 2, 2, 1}
)

// weights are the model's trained values, read-only and shared by every
// stream. The recurrent cell's rows are PyTorch's gate order: input, forget,
// cell, output.
type weights struct {
	// spectrum is the fast transform the stored basis amounts to.
	spectrum   *fourier
	encoder    [4][]float32
	encoderB   [4][]float32
	inputGates []float32 // [512][128]
	stateGates []float32 // [512][128]
	gateBias   []float32 // [512]: both biases added
	head       []float32 // [128]
	headB      float32
}

// shapes are the tensors the network needs, as the model names them.
var shapes = map[string][]int{
	"model.stft.forward_basis_buffer":     {2 * stftBins, 1, stftWindow},
	"model.encoder.0.reparam_conv.weight": {128, stftBins, 3},
	"model.encoder.0.reparam_conv.bias":   {128},
	"model.encoder.1.reparam_conv.weight": {64, 128, 3},
	"model.encoder.1.reparam_conv.bias":   {64},
	"model.encoder.2.reparam_conv.weight": {64, 64, 3},
	"model.encoder.2.reparam_conv.bias":   {64},
	"model.encoder.3.reparam_conv.weight": {128, 64, 3},
	"model.encoder.3.reparam_conv.bias":   {128},
	"model.decoder.rnn.weight_ih":         {4 * hidden, hidden},
	"model.decoder.rnn.weight_hh":         {4 * hidden, hidden},
	"model.decoder.rnn.bias_ih":           {4 * hidden},
	"model.decoder.rnn.bias_hh":           {4 * hidden},
	"model.decoder.decoder.2.weight":      {1, hidden, 1},
	"model.decoder.decoder.2.bias":        {1},
}

// weightsOf picks the network's tensors, each of exactly its shape.
func weightsOf(tensors map[string]tensor) (*weights, error) {
	for name, want := range shapes {
		t, found := tensors[name]
		if !found {
			return nil, fmt.Errorf("%w: tensor %q missing", errMalformed, name)
		}
		if !sameShape(t.shape, want) {
			return nil, fmt.Errorf("%w: tensor %q of shape %v, want %v", errMalformed, name, t.shape, want)
		}
	}
	spectrum, err := fourierOf(tensors["model.stft.forward_basis_buffer"].data)
	if err != nil {
		return nil, err
	}
	w := &weights{
		spectrum:   spectrum,
		inputGates: tensors["model.decoder.rnn.weight_ih"].data,
		stateGates: tensors["model.decoder.rnn.weight_hh"].data,
		head:       tensors["model.decoder.decoder.2.weight"].data,
		headB:      tensors["model.decoder.decoder.2.bias"].data[0],
		gateBias:   make([]float32, 4*hidden),
	}
	inputBias, stateBias := tensors["model.decoder.rnn.bias_ih"].data, tensors["model.decoder.rnn.bias_hh"].data
	for i := range w.gateBias {
		w.gateBias[i] = inputBias[i] + stateBias[i]
	}
	for i := range w.encoder {
		w.encoder[i] = tensors[fmt.Sprintf("model.encoder.%d.reparam_conv.weight", i)].data
		w.encoderB[i] = tensors[fmt.Sprintf("model.encoder.%d.reparam_conv.bias", i)].data
	}
	return w, nil
}

func sameShape(a, b []int) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}
	return true
}

// stream is one stretch of sound being judged: the context and the cell's
// state it carries, and room for every step, so a chunk allocates nothing.
type stream struct {
	w       *weights
	context [contextSamples]float32
	h, c    [hidden]float32
	padded  [inputSamples + stftPad]float32
	// layers[0] is the spectrum, [bin][frame]; layers[i+1] the output of
	// the encoder's convolution i, [channel][frame].
	layers  [5][]float32
	columns []float32 // one frame's inputs to a convolution, kernel by kernel
	gates   [4 * hidden]float32
}

func newStream(w *weights) *stream {
	s := &stream{w: w}
	frames := stftFrames
	s.layers[0] = make([]float32, stftBins*frames)
	widest := 0
	for i := range encoderStrides {
		widest = max(widest, encoderChannels[i]*3)
		frames = (frames-1)/encoderStrides[i] + 1
		s.layers[i+1] = make([]float32, encoderChannels[i+1]*frames)
	}
	s.columns = make([]float32, widest)
	return s
}

// Speech judges the next chunk: the chance it is speech.
func (s *stream) Speech(chunk []float32) float32 {
	if len(chunk) != speech.ChunkSamples {
		return 0
	}
	input := s.padded[:inputSamples]
	copy(input, s.context[:])
	copy(input[contextSamples:], chunk)
	copy(s.context[:], input[inputSamples-contextSamples:])
	// The end is mirrored, leaving out the last sample: p[576+j] = x[574−j].
	for j := range stftPad {
		s.padded[inputSamples+j] = input[inputSamples-2-j]
	}
	s.spectrum()
	frames := stftFrames
	for i := range encoderStrides {
		frames = s.convolve(i, frames)
	}
	return s.decide(s.layers[4])
}

// spectrum is the magnitude of each frequency bin in each frame, [bin][frame].
func (s *stream) spectrum() {
	for frame := range stftFrames {
		s.w.spectrum.magnitudes(s.padded[frame*stftHop:], s.layers[0][frame:], stftFrames)
	}
}

// convolve runs the encoder's convolution i, with its ReLU, over frames
// frames of the layer before, and returns how many frames come out.
func (s *stream) convolve(i, frames int) int {
	in, out := s.layers[i], s.layers[i+1]
	inChannels, outChannels, stride := encoderChannels[i], encoderChannels[i+1], encoderStrides[i]
	outFrames := (frames-1)/stride + 1
	columns := s.columns[:inChannels*3]
	for t := range outFrames {
		// The kernel's three frames around t·stride, zero past the edges.
		first := t*stride - 1
		for c := range inChannels {
			row := in[c*frames : (c+1)*frames]
			for k := range 3 {
				v := float32(0)
				if f := first + k; f >= 0 && f < frames {
					v = row[f]
				}
				columns[c*3+k] = v
			}
		}
		weights := s.w.encoder[i]
		for o := range outChannels {
			v := s.w.encoderB[i][o] + dot(weights[o*inChannels*3:(o+1)*inChannels*3], columns)
			out[o*outFrames+t] = max(v, 0)
		}
	}
	return outFrames
}

// decide runs the recurrent cell over the encoder's output and the head
// over its new state.
func (s *stream) decide(x []float32) float32 {
	for r := range s.gates {
		s.gates[r] = s.w.gateBias[r] + dot(s.w.inputGates[r*hidden:(r+1)*hidden], x) +
			dot(s.w.stateGates[r*hidden:(r+1)*hidden], s.h[:])
	}
	for j := range s.h {
		in := sigmoid(s.gates[j])
		forget := sigmoid(s.gates[hidden+j])
		cell := tanh(s.gates[2*hidden+j])
		out := sigmoid(s.gates[3*hidden+j])
		s.c[j] = forget*s.c[j] + in*cell
		s.h[j] = out * tanh(s.c[j])
	}
	// The head sees the state through a ReLU; the state goes on as it is.
	v := s.w.headB
	for j, h := range s.h {
		v += s.w.head[j] * max(h, 0)
	}
	return sigmoid(v)
}

// dot is the dot product of two equally long vectors, in four running sums
// so they do not wait on each other (eight sums measured no faster).
func dot(a, b []float32) float32 {
	b = b[:len(a)]
	var s0, s1, s2, s3 float32
	i := 0
	for ; i+4 <= len(a); i += 4 {
		s0 += a[i] * b[i]
		s1 += a[i+1] * b[i+1]
		s2 += a[i+2] * b[i+2]
		s3 += a[i+3] * b[i+3]
	}
	for ; i < len(a); i++ {
		s0 += a[i] * b[i]
	}
	return (s0 + s1) + (s2 + s3)
}

func sigmoid(x float32) float32 { return float32(1 / (1 + math.Exp(-float64(x)))) }

func tanh(x float32) float32 { return float32(math.Tanh(float64(x))) }
