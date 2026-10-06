package media_test

import (
	"testing"

	"github.com/ediejohnny/lingoreel/internal/domain/media"
)

// Only a file that starts as the container its extension names is a video:
// a page, a script or a program renamed "video.mp4" is not, nor is anything
// with an extension the app does not open.
func TestOnlyTheRealContainerMatchesItsName(t *testing.T) {
	t.Parallel()
	const size = 1 << 30
	var (
		matroska  = "\x1a\x45\xdf\xa3\x9f\x42\x86\x81\x01\x42\xf7\x81\x01\x42\xf2\x81"
		mp4       = "\x00\x00\x00\x18ftypisom\x00\x00\x02\x00"
		quicktime = "\x00\x00\x00\x08wide\x00\x00\x00\x00mdat"
		avi       = "RIFF\x00\x10\x00\x00AVI LIST"
		asf       = "\x30\x26\xb2\x75\x8e\x66\xcf\x11\xa6\xd9\x00\xaa\x00\x62\xce\x6c"
		script    = "alert('hacked'); require('child_process').exec('calc')"
		page      = "<!DOCTYPE html><html><script>alert(1)</script>"
		program   = "MZ\x90\x00\x03\x00\x00\x00\x04\x00\x00\x00\xff\xff\x00\x00"
		shell     = "#!/bin/sh\nrm -rf ~\n"
		lucky     = "var mdat = 1;\nvar x = 2;" // "mdat" where an MP4's first box is named
	)
	cases := []struct {
		extension, head string
		want            bool
	}{
		{".mkv", matroska, true}, {".webm", matroska, true}, {".MKV", matroska, true},
		{".mp4", mp4, true}, {".m4v", mp4, true}, {".mov", quicktime, true},
		{".avi", avi, true}, {".wmv", asf, true},
		{".mkv", script, false}, {".mp4", script, false}, {".avi", script, false}, {".wmv", script, false},
		{".mkv", page, false}, {".mov", page, false},
		{".mp4", program, false}, {".avi", program, false},
		{".mkv", shell, false},
		{".mp4", lucky, false},
		{".mp4", avi, false}, {".avi", mp4, false}, {".mkv", asf, false},
		{".mp4", "", false},
		{".exe", program, false}, {".html", page, false}, {".js", script, false}, {".sh", shell, false}, {"", matroska, false},
	}
	for _, c := range cases {
		if got := media.MatchesContainer(c.extension, []byte(c.head), size); got != c.want {
			t.Errorf("MatchesContainer(%q, %.12q) = %v, want %v", c.extension, c.head, got, c.want)
		}
	}
	if media.MatchesContainer(".mp4", []byte(mp4), 16) {
		t.Error("an MP4 whose first box is larger than the file matched")
	}
}
