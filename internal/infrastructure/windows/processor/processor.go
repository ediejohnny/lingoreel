// Package processor tells the computer's processor as Windows names it, to
// tell one computer from another and to show what the app read of it.
package processor

import (
	"golang.org/x/sys/windows/registry"

	"github.com/ediejohnny/lingoreel/internal/domain/capacity"
)

// key is where Windows describes the first processor.
const key = `HARDWARE\DESCRIPTION\System\CentralProcessor\0`

// Name is the processor's name, such as "Intel(R) Core(TM) i5-2400 CPU @
// 3.10GHz", as capacity.ProcessorName keeps it; empty when Windows cannot
// tell.
func Name() string {
	described, err := registry.OpenKey(registry.LOCAL_MACHINE, key, registry.QUERY_VALUE)
	if err != nil {
		return ""
	}
	defer described.Close()
	name, _, err := described.GetStringValue("ProcessorNameString")
	if err != nil {
		return ""
	}
	return capacity.ProcessorName(name)
}
