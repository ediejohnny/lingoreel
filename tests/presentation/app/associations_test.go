package app_test

import (
	"errors"
	"strings"
	"testing"
)

// fakeAssociations stands for the registry: it keeps the state and can fail.
type fakeAssociations struct {
	on      bool
	readErr error
	setErr  error
	calls   []string
}

func (f *fakeAssociations) Enabled() (bool, error) { return f.on, f.readErr }
func (f *fakeAssociations) Enable() error {
	f.calls = append(f.calls, "enable")
	if f.setErr != nil {
		return f.setErr
	}
	f.on = true
	return nil
}
func (f *fakeAssociations) Disable() error {
	f.calls = append(f.calls, "disable")
	if f.setErr != nil {
		return f.setErr
	}
	f.on = false
	return nil
}

func TestTheWindowsAssociationIsTheUsersChoice(t *testing.T) {
	t.Parallel()
	r := newRig(t)
	r.attach(t)
	if on, err := r.api.FileAssociation(); on || err != nil {
		t.Fatalf("FileAssociation at first = %v, %v", on, err)
	}
	if err := r.api.SetFileAssociation(true); err != nil {
		t.Fatal(err)
	}
	if on, _ := r.api.FileAssociation(); !on {
		t.Fatal("not on after turning it on")
	}
	if err := r.api.SetFileAssociation(false); err != nil {
		t.Fatal(err)
	}
	if got := strings.Join(r.associations.calls, ","); got != "enable,disable" {
		t.Fatalf("calls = %s", got)
	}

	r.associations.setErr = errors.New("access denied")
	wantMessage(t, "SetFileAssociation failing", r.api.SetFileAssociation(true), "Não foi possível mudar como o Windows abre os vídeos")
	r.associations.readErr = errors.New("access denied")
	if _, err := r.api.FileAssociation(); err == nil {
		t.Fatal("a failed read was not reported")
	}
}
