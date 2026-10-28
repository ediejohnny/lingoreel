package playerkit

import (
	"fmt"
	"os"
	"path/filepath"
)

// CheckLaunch checks a launch's executable and video before the player
// starts: both are existing regular files, the executable given by an
// absolute path. It returns the video's absolute path, which starts with a
// drive or a share, never with "-" or "/", so no player takes the file's
// name for one of its options.
func CheckLaunch(name, executable, video string) (string, error) {
	if !filepath.IsAbs(executable) || !isRegularFile(executable) {
		return "", fmt.Errorf("%s executable %q not found", name, executable)
	}
	file, err := filepath.Abs(video)
	if err != nil || !isRegularFile(file) {
		return "", fmt.Errorf("video %q not found", video)
	}
	return file, nil
}

func isRegularFile(path string) bool {
	info, err := os.Stat(path)
	return err == nil && info.Mode().IsRegular()
}
