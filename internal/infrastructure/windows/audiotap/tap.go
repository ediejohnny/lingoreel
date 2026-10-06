//go:build windows

// Package audiotap hears what a player plays, through WASAPI's loopback:
// only the player's own sound when Windows allows it (process loopback,
// Windows 10 2004 and later), or everything the speakers play.
package audiotap

import (
	"context"
	"errors"
	"fmt"
	"time"
	"unsafe"

	"golang.org/x/sys/windows"

	"github.com/ediejohnny/lingoreel/internal/application/voice"
	"github.com/ediejohnny/lingoreel/internal/domain/speech"
)

// The sound asked of Windows, which converts it: what the Analyzer takes,
// in stereo so the voice in the middle can be told from what is spread wide.
const (
	channels       = 2
	bytesPerSample = 4
	// bufferLength gives the loop room for a pause of the garbage
	// collector, in 100 ns units: 200 ms.
	bufferLength = 2_000_000
	// pollEvery is the longest wait for a packet: the event that says one
	// is ready is not raised in every case of loopback.
	pollEvery = 20 // ms
)

// waveFormat is WAVEFORMATEX.
type waveFormat struct {
	tag            uint16
	channels       uint16
	samplesPerSec  uint32
	avgBytesPerSec uint32
	blockAlign     uint16
	bitsPerSample  uint16
	size           uint16
}

// Tap hears a player process, implementing voice.Tap.
type Tap struct {
	// Speakers hears everything the speakers play even where Windows could
	// hear the player alone: for checking that way on a machine.
	Speakers bool
}

var _ voice.Tap = Tap{}

// Listen hears the process with pid until ctx ends. When Windows cannot
// hear the process alone, it hears every sound the speakers play.
func (t Tap) Listen(ctx context.Context, pid int, hear func(voice.Packet)) error {
	done, err := initCOM()
	if err != nil {
		return err
	}
	defer done()
	client, system, err := open(uint32(pid), t.Speakers)
	if err != nil {
		return err
	}
	defer client.release()
	return capture(ctx, client, system, hear)
}

// open starts an audio client on the process, or on the speakers.
func open(pid uint32, speakers bool) (client *object, system bool, err error) {
	format := waveFormat{
		tag: waveFloat, channels: channels, samplesPerSec: speech.SampleRate,
		avgBytesPerSec: speech.SampleRate * channels * bytesPerSample, blockAlign: channels * bytesPerSample,
		bitsPerSample: 8 * bytesPerSample,
	}
	flags := uintptr(streamLoopback | streamEventCallback | streamAutoConvert | streamSRCDefault)
	processErr := errors.New("not tried")
	if !speakers {
		var process *object
		if process, processErr = processLoopback(pid); processErr == nil {
			if processErr = process.call(clientInitialize, shareModeShared, flags, bufferLength, 0, uintptr(unsafe.Pointer(&format)), 0); processErr == nil {
				return process, false, nil
			}
			process.release()
		}
	}
	everything, err := systemLoopback()
	if err != nil {
		return nil, false, fmt.Errorf("%w (the player alone: %v)", err, processErr)
	}
	if err := everything.call(clientInitialize, shareModeShared, flags, bufferLength, 0, uintptr(unsafe.Pointer(&format)), 0); err != nil {
		everything.release()
		return nil, false, fmt.Errorf("starting to hear the speakers: %w (the player alone: %v)", err, processErr)
	}
	return everything, true, nil
}

// capture hands every packet to hear until ctx ends.
func capture(ctx context.Context, client *object, system bool, hear func(voice.Packet)) error {
	event, err := windows.CreateEvent(nil, 0, 0, nil)
	if err != nil {
		return fmt.Errorf("creating the capture event: %w", err)
	}
	defer windows.CloseHandle(event)
	if err := client.call(clientSetEventHandle, uintptr(event)); err != nil {
		return fmt.Errorf("setting the capture event: %w", err)
	}
	var reader *object
	if err := client.call(clientGetService, uintptr(unsafe.Pointer(&iidAudioCaptureClient)), uintptr(unsafe.Pointer(&reader))); err != nil {
		return fmt.Errorf("opening the capture: %w", err)
	}
	defer reader.release()
	if err := client.call(clientStart); err != nil {
		return fmt.Errorf("starting the capture: %w", err)
	}
	defer client.call(clientStop)

	clock := newQPCClock()
	var samples []float32
	for ctx.Err() == nil {
		if _, err := windows.WaitForSingleObject(event, pollEvery); err != nil {
			return fmt.Errorf("waiting for sound: %w", err)
		}
		for {
			var next uint32
			if err := reader.call(captureGetNextPacketSize, uintptr(unsafe.Pointer(&next))); err != nil {
				return fmt.Errorf("reading the sound: %w", err)
			}
			if next == 0 {
				break
			}
			var data *byte
			var frames, flags uint32
			var position, stamp uint64
			if err := reader.call(captureGetBuffer, uintptr(unsafe.Pointer(&data)), uintptr(unsafe.Pointer(&frames)),
				uintptr(unsafe.Pointer(&flags)), uintptr(unsafe.Pointer(&position)), uintptr(unsafe.Pointer(&stamp))); err != nil {
				return fmt.Errorf("reading the sound: %w", err)
			}
			n := int(frames * channels)
			if cap(samples) < n {
				samples = make([]float32, n)
			}
			samples = samples[:n]
			if flags&bufferSilent != 0 || data == nil {
				clear(samples)
			} else {
				copy(samples, unsafe.Slice((*float32)(unsafe.Pointer(data)), n))
			}
			if err := reader.call(captureReleaseBuffer, uintptr(frames)); err != nil {
				return fmt.Errorf("reading the sound: %w", err)
			}
			at := clock.at(stamp)
			if flags&bufferTimestampError != 0 || stamp == 0 {
				at = time.Now()
			}
			hear(voice.Packet{Samples: samples, Channels: channels, System: system, At: at})
		}
	}
	return nil
}
