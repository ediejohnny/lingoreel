package onnx_test

import (
	"errors"
	"os"
	"path/filepath"
	"slices"
	"testing"

	"github.com/ediejohnny/lingoreel/internal/infrastructure/onnx"
)

// The runtime is an optional download, not in the repository: these tests
// run when LR_ONNXRUNTIME names an onnxruntime.dll. testdata/double.onnx
// doubles its input "x" into "y".
func runtimeOrSkip(t *testing.T) *onnx.Runtime {
	t.Helper()
	path := os.Getenv("LR_ONNXRUNTIME")
	if path == "" {
		t.Skip("LR_ONNXRUNTIME names no onnxruntime.dll")
	}
	r, err := onnx.Load(path)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(r.Close)
	return r
}

func TestAModelRunsThroughTheRuntime(t *testing.T) {
	r := runtimeOrSkip(t)
	model, err := filepath.Abs(filepath.Join("testdata", "double.onnx"))
	if err != nil {
		t.Fatal(err)
	}
	session, err := r.Open(model, "x", "y", 2)
	if err != nil {
		t.Fatal(err)
	}
	defer session.Close()
	for _, input := range [][]float32{{1, 2, 3}, {-0.5, 0, 0.25, 8, 100}} {
		out, shape, err := session.Run(input, []int64{1, int64(len(input))})
		if err != nil {
			t.Fatal(err)
		}
		want := make([]float32, len(input))
		for i, v := range input {
			want[i] = 2 * v
		}
		if !slices.Equal(out, want) || !slices.Equal(shape, []int64{1, int64(len(input))}) {
			t.Errorf("Run(%v) = %v %v, want %v", input, out, shape, want)
		}
	}
	if r.Version() == "" {
		t.Error("the runtime told no version")
	}
}

func TestWhatTheRuntimeRefusesSaysWhy(t *testing.T) {
	if _, err := onnx.Load(filepath.Join(t.TempDir(), "onnxruntime.dll")); err == nil {
		t.Error("loading a missing DLL worked")
	}
	if _, err := onnx.Load(`C:\Windows\System32\kernel32.dll`); !errors.Is(err, onnx.ErrRuntime) {
		t.Errorf("loading kernel32 as the runtime: %v, want ErrRuntime", err)
	}
	r := runtimeOrSkip(t)
	if _, err := r.Open(filepath.Join(t.TempDir(), "none.onnx"), "x", "y", 1); err == nil {
		t.Error("opening a missing model worked")
	}
}
