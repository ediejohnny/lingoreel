// Package power tells the voice when the computer runs on its battery, from
// Windows' power status.
package power

import (
	"context"
	"time"
	"unsafe"

	"golang.org/x/sys/windows"

	"github.com/ediejohnny/lingoreel/internal/application/voice"
)

var procGetSystemPowerStatus = windows.NewLazySystemDLL("kernel32.dll").NewProc("GetSystemPowerStatus")

// systemPowerStatus is Windows' SYSTEM_POWER_STATUS.
type systemPowerStatus struct {
	ACLineStatus        byte
	BatteryFlag         byte
	BatteryLifePercent  byte
	SystemStatusFlag    byte
	BatteryLifeTime     uint32
	BatteryFullLifeTime uint32
}

const (
	acOffline      = 0   // ACLineStatus: on the battery
	noBattery      = 128 // BatteryFlag: no system battery
	unknownBattery = 255 // BatteryFlag: the status cannot be read
)

// pollEvery is how often the status is read: Windows has no cheaper way
// for a program without a window to hear of the change, and a change seen
// within 20 s is soon enough to rest the voice model.
const pollEvery = 20 * time.Second

// Status reads Windows' power status, implementing voice.Power.
type Status struct{}

var _ voice.Power = Status{}

func read() (systemPowerStatus, bool) {
	var status systemPowerStatus
	ok, _, _ := procGetSystemPowerStatus.Call(uintptr(unsafe.Pointer(&status)))
	return status, ok != 0
}

// HasBattery tells whether the computer has a battery at all.
func (Status) HasBattery() bool {
	status, ok := read()
	return ok && status.BatteryFlag != noBattery && status.BatteryFlag != unknownBattery
}

// OnBattery tells whether the computer runs on its battery now; when
// Windows cannot tell, it is taken as plugged in.
func (Status) OnBattery() bool {
	status, ok := read()
	return ok && status.ACLineStatus == acOffline
}

// Watch reads the status every pollEvery and calls changed when the
// computer went on or off its battery, until ctx ends.
func (s Status) Watch(ctx context.Context, changed func(onBattery bool)) {
	last := s.OnBattery()
	ticker := time.NewTicker(pollEvery)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			if now := s.OnBattery(); now != last {
				last = now
				changed(now)
			}
		}
	}
}
