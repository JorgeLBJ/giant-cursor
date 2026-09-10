package cursor

import (
	"errors"
	"testing"
)

type fakeEffector struct {
	enlarged, restored int
	err                error
}

func (f *fakeEffector) Enlarge() error { f.enlarged++; return f.err }
func (f *fakeEffector) Restore() error { f.restored++; return f.err }

type fakeTrackingEffector struct {
	fakeEffector
	tracked      int
	lastX, lastY int
}

func (f *fakeTrackingEffector) Track(x, y int) { f.tracked++; f.lastX, f.lastY = x, y }

func TestMultiFansOutToEveryMember(t *testing.T) {
	a, b := &fakeEffector{}, &fakeEffector{}
	m := Multi{a, b}

	if err := m.Enlarge(); err != nil {
		t.Fatalf("Enlarge = %v, want nil", err)
	}
	if err := m.Restore(); err != nil {
		t.Fatalf("Restore = %v, want nil", err)
	}
	if a.enlarged != 1 || b.enlarged != 1 {
		t.Errorf("Enlarge reached members %d and %d times, want 1 and 1", a.enlarged, b.enlarged)
	}
	if a.restored != 1 || b.restored != 1 {
		t.Errorf("Restore reached members %d and %d times, want 1 and 1", a.restored, b.restored)
	}
}

func TestMultiAttemptsEveryMemberAfterAnError(t *testing.T) {
	boom := errors.New("boom")
	failing, healthy := &fakeEffector{err: boom}, &fakeEffector{}
	m := Multi{failing, healthy}

	if err := m.Enlarge(); !errors.Is(err, boom) {
		t.Fatalf("Enlarge = %v, want boom", err)
	}
	if healthy.enlarged != 1 {
		t.Error("a failing member must not stop the members after it")
	}
}

func TestMultiReturnsTheFirstError(t *testing.T) {
	first, second := errors.New("first"), errors.New("second")
	m := Multi{&fakeEffector{err: first}, &fakeEffector{err: second}}

	if err := m.Restore(); !errors.Is(err, first) {
		t.Fatalf("Restore = %v, want the first error", err)
	}
}

func TestMultiTracksOnlyTrackingMembers(t *testing.T) {
	plain, tracker := &fakeEffector{}, &fakeTrackingEffector{}
	m := Multi{plain, tracker}

	m.Track(120, 340)

	if tracker.tracked != 1 {
		t.Fatalf("tracked %d times, want 1", tracker.tracked)
	}
	if tracker.lastX != 120 || tracker.lastY != 340 {
		t.Errorf("tracked (%d,%d), want (120,340)", tracker.lastX, tracker.lastY)
	}
}
