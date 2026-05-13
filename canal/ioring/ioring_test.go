package ioring

import (
	"sync"
	"testing"
)

// ─── Submit / PollSQ ─────────────────────────────────────────────────────────

func TestSubmitAndPollSQ(t *testing.T) {
	r := New()
	d := Descriptor{Op: 1, Service: 2, Cookie: 99}

	if err := r.Submit(d); err != nil {
		t.Fatalf("Submit: %v", err)
	}

	got, err := r.PollSQ()
	if err != nil {
		t.Fatalf("PollSQ: %v", err)
	}
	if got != d {
		t.Fatalf("PollSQ = %+v, want %+v", got, d)
	}
}

func TestPollSQEmptyReturnsError(t *testing.T) {
	r := New()
	if _, err := r.PollSQ(); err != ErrRingEmpty {
		t.Fatalf("got %v, want ErrRingEmpty", err)
	}
}

func TestSubmitFIFOOrder(t *testing.T) {
	r := New()
	for i := uint64(0); i < 4; i++ {
		if err := r.Submit(Descriptor{Cookie: i}); err != nil {
			t.Fatalf("Submit %d: %v", i, err)
		}
	}
	for i := uint64(0); i < 4; i++ {
		d, err := r.PollSQ()
		if err != nil {
			t.Fatalf("PollSQ %d: %v", i, err)
		}
		if d.Cookie != i {
			t.Fatalf("PollSQ[%d].Cookie = %d, want %d", i, d.Cookie, i)
		}
	}
}

func TestSubmitFullReturnsError(t *testing.T) {
	r := New()
	for i := 0; i < RingSize; i++ {
		if err := r.Submit(Descriptor{Cookie: uint64(i)}); err != nil {
			t.Fatalf("Submit %d: %v", i, err)
		}
	}
	if err := r.Submit(Descriptor{}); err != ErrRingFull {
		t.Fatalf("got %v, want ErrRingFull", err)
	}
}

// ─── Complete / PollCQ ───────────────────────────────────────────────────────

func TestCompleteAndPollCQ(t *testing.T) {
	r := New()
	d := Descriptor{Cookie: 7, Status: 0}

	if err := r.Complete(d); err != nil {
		t.Fatalf("Complete: %v", err)
	}

	got, err := r.PollCQ()
	if err != nil {
		t.Fatalf("PollCQ: %v", err)
	}
	if got != d {
		t.Fatalf("PollCQ = %+v, want %+v", got, d)
	}
}

func TestPollCQEmptyReturnsError(t *testing.T) {
	r := New()
	if _, err := r.PollCQ(); err != ErrRingEmpty {
		t.Fatalf("got %v, want ErrRingEmpty", err)
	}
}

func TestCompleteFIFOOrder(t *testing.T) {
	r := New()
	for i := uint64(0); i < 4; i++ {
		if err := r.Complete(Descriptor{Cookie: i}); err != nil {
			t.Fatalf("Complete %d: %v", i, err)
		}
	}
	for i := uint64(0); i < 4; i++ {
		d, err := r.PollCQ()
		if err != nil {
			t.Fatalf("PollCQ %d: %v", i, err)
		}
		if d.Cookie != i {
			t.Fatalf("PollCQ[%d].Cookie = %d, want %d", i, d.Cookie, i)
		}
	}
}

// ─── SubmitAndWait (blocking bridge) ─────────────────────────────────────────

func TestSubmitAndWaitMatchesCookie(t *testing.T) {
	r := New()

	req := Descriptor{Op: 3, Service: 1, Cookie: 0xABCD}

	// Simulate the service processing the SQE and posting the CQE.
	var wg sync.WaitGroup
	wg.Add(1)
	go func() {
		defer wg.Done()
		r.WaitSQ()
		sqe, err := r.PollSQ()
		if err != nil {
			t.Errorf("service PollSQ: %v", err)
			return
		}
		if err := r.Complete(Descriptor{Cookie: sqe.Cookie, Status: 0}); err != nil {
			t.Errorf("service Complete: %v", err)
		}
	}()

	cqe, err := r.SubmitAndWait(req)
	wg.Wait()

	if err != nil {
		t.Fatalf("SubmitAndWait: %v", err)
	}
	if cqe.Cookie != req.Cookie {
		t.Fatalf("CQE cookie = %d, want %d", cqe.Cookie, req.Cookie)
	}
	if cqe.Status != 0 {
		t.Fatalf("CQE status = %d, want 0", cqe.Status)
	}
}

// TestRingWrapAround verifies that the ring buffer wraps correctly after
// exceeding RingSize total operations.
func TestRingWrapAround(t *testing.T) {
	r := New()
	const rounds = RingSize * 3

	for i := uint64(0); i < rounds; i++ {
		if err := r.Submit(Descriptor{Cookie: i}); err != nil {
			t.Fatalf("Submit %d: %v", i, err)
		}
		got, err := r.PollSQ()
		if err != nil {
			t.Fatalf("PollSQ %d: %v", i, err)
		}
		if got.Cookie != i {
			t.Fatalf("PollSQ[%d].Cookie = %d, want %d", i, got.Cookie, i)
		}
	}
}
