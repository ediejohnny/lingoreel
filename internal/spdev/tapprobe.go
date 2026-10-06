package spdev

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"io"
	"math"
	"time"

	"github.com/ediejohnny/lingoreel/internal/application/voice"
	"github.com/ediejohnny/lingoreel/internal/domain/speech"
	"github.com/ediejohnny/lingoreel/internal/infrastructure/silero"
	"github.com/ediejohnny/lingoreel/internal/infrastructure/windows/audiotap"
)

// tapProbe hears a player's sound for a while and prints, second by second,
// what came: whether Windows heard the player alone, the packets, the
// samples (16000 a second when all is well), each side's loudness, the share
// heard as voice and how late the packets arrived. It guides the check of
// the capture on each machine.
func tapProbe(ctx context.Context, args []string, stdout io.Writer) error {
	flags := flag.NewFlagSet("tap-probe", flag.ContinueOnError)
	flags.SetOutput(stdout)
	pid := flags.Int("pid", 0, "the player's process id")
	seconds := flags.Int("seconds", 10, "how long to listen")
	speakers := flags.Bool("speakers", false, "hear everything the speakers play, as when Windows cannot hear the player alone")
	if err := flags.Parse(args); err != nil {
		return err
	}
	if *pid <= 0 {
		return errors.New("tap-probe: --pid is required")
	}
	ctx, cancel := context.WithTimeout(ctx, time.Duration(*seconds)*time.Second)
	defer cancel()

	out := &printer{w: stdout}
	stream, err := silero.Embedded{}.NewStream()
	if err != nil {
		return err
	}
	analyzer, err := speech.NewAnalyzer(2, stream)
	if err != nil {
		return err
	}
	listener := speech.NewListener(speech.SensitivityNormal)
	var (
		frames                  []speech.Frame
		second                  time.Time
		packets, samples        int
		left, right             float64
		voiced, sure, heard     int
		lastPacket              time.Time
		longestGap, latenessSum time.Duration
		told                    bool
	)
	report := func() {
		if packets == 0 {
			return
		}
		per := float64(max(samples, 1))
		out.line("%s  packets %3d  samples %5d  L %6.1f dB  R %6.1f dB  voice %3.0f%%  sure %3.0f%%  longest gap %4d ms  late %3d ms",
			second.Format("15:04:05"), packets, samples, decibels(left/per), decibels(right/per),
			100*float64(voiced)/float64(max(heard, 1)), 100*float64(sure)/float64(max(heard, 1)),
			longestGap.Milliseconds(), (latenessSum / time.Duration(packets)).Milliseconds())
		packets, samples, left, right, voiced, sure, heard, longestGap, latenessSum = 0, 0, 0, 0, 0, 0, 0, 0, 0
	}
	hear := func(packet voice.Packet) {
		if !told {
			told = true
			if packet.System {
				out.line("Hearing everything the speakers play: Windows could not hear the player alone.")
			} else {
				out.line("Hearing the player alone.")
			}
		}
		if packet.At.Sub(second) >= time.Second {
			report()
			second = packet.At
		}
		if !lastPacket.IsZero() {
			longestGap = max(longestGap, packet.At.Sub(lastPacket))
		}
		lastPacket = packet.At
		packets++
		latenessSum += time.Since(packet.At)
		for i := 0; i+1 < len(packet.Samples); i += 2 {
			l, r := float64(packet.Samples[i]), float64(packet.Samples[i+1])
			left += l * l
			right += r * r
			samples++
		}
		frames = analyzer.Write(packet.Samples, frames[:0])
		for _, frame := range frames {
			isVoice, isSure := listener.Hear(frame)
			heard++
			if isVoice {
				voiced++
			}
			if isSure {
				sure++
			}
		}
	}
	out.line("Listening to process %d for %d s...", *pid, *seconds)
	err = audiotap.Tap{Speakers: *speakers}.Listen(ctx, *pid, hear)
	report()
	if !told && err == nil {
		out.line("No sound came: the player was silent, paused, or its sound goes straight to the receiver.")
	}
	if err != nil {
		return fmt.Errorf("tap-probe: %w", err)
	}
	return out.err
}

func decibels(meanSquare float64) float64 {
	return max(10*math.Log10(meanSquare), -120)
}
