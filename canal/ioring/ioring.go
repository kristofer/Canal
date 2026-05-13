// Package ioring implements an io_uring-style submission/completion queue pair
// for typed inter-domain request transport in Canal.
//
// Each domain owns one Ring. The domain writes Descriptors to the SQ; the
// service reads them, executes the operation, and posts Descriptors to the CQ.
// SubmitAndWait is a convenience bridge that lets existing blocking callers use
// the SQ/CQ path transparently.
package ioring

import (
	"errors"
	"sync"
)

// RingSize is the fixed capacity of each SQ and CQ.  Must be a power of two.
const RingSize = 16

// Descriptor is a submission-queue entry (SQE) or completion-queue entry (CQE).
//
// For an SQE: Op, Service, Cookie, ReqPtr, and ReqLen are populated.
// For a CQE: Cookie, Status, RespPtr, and RespLen are populated.
type Descriptor struct {
	Op      uint8   // service-specific operation enum (e.g. channel.FSOpRead)
	Service uint8   // channel.ServiceID
	Cookie  uint64  // caller correlation ID; echoed unchanged in the CQE
	ReqPtr  uintptr // pointer to the typed request struct
	ReqLen  uint32  // size of the request struct in bytes
	Status  uint8   // completion status (0 = success)
	RespPtr uintptr // pointer to the typed response struct (CQE only)
	RespLen uint32  // size of the response struct in bytes (CQE only)
}

// Ring holds one domain's submission queue (SQ) and completion queue (CQ).
// It is safe for concurrent use by a single producer on each side.
type Ring struct {
	mu   sync.Mutex
	sq   [RingSize]Descriptor
	sqH  uint32 // SQ head — advanced by the service consumer
	sqT  uint32 // SQ tail — advanced by the domain producer
	cq   [RingSize]Descriptor
	cqH  uint32 // CQ head — advanced by the domain consumer
	cqT  uint32 // CQ tail — advanced by the service producer
	sqCh chan struct{}
	cqCh chan struct{}
}

var (
	ErrRingFull  = errors.New("ioring: ring full")
	ErrRingEmpty = errors.New("ioring: ring empty")
)

// New returns an initialised Ring.
func New() *Ring {
	return &Ring{
		sqCh: make(chan struct{}, RingSize),
		cqCh: make(chan struct{}, RingSize),
	}
}

// Submit enqueues d onto the SQ.  Returns ErrRingFull if the SQ is full.
func (r *Ring) Submit(d Descriptor) error {
	r.mu.Lock()
	if r.sqT-r.sqH >= RingSize {
		r.mu.Unlock()
		return ErrRingFull
	}
	r.sq[r.sqT&(RingSize-1)] = d
	r.sqT++
	r.mu.Unlock()
	// Non-blocking notify.
	select {
	case r.sqCh <- struct{}{}:
	default:
	}
	return nil
}

// PollSQ removes and returns the next SQE, or ErrRingEmpty.
func (r *Ring) PollSQ() (Descriptor, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	if r.sqH == r.sqT {
		return Descriptor{}, ErrRingEmpty
	}
	d := r.sq[r.sqH&(RingSize-1)]
	r.sqH++
	return d, nil
}

// Complete enqueues d onto the CQ.  Returns ErrRingFull if the CQ is full.
func (r *Ring) Complete(d Descriptor) error {
	r.mu.Lock()
	if r.cqT-r.cqH >= RingSize {
		r.mu.Unlock()
		return ErrRingFull
	}
	r.cq[r.cqT&(RingSize-1)] = d
	r.cqT++
	r.mu.Unlock()
	select {
	case r.cqCh <- struct{}{}:
	default:
	}
	return nil
}

// PollCQ removes and returns the next CQE, or ErrRingEmpty.
func (r *Ring) PollCQ() (Descriptor, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	if r.cqH == r.cqT {
		return Descriptor{}, ErrRingEmpty
	}
	d := r.cq[r.cqH&(RingSize-1)]
	r.cqH++
	return d, nil
}

// WaitSQ blocks until at least one SQE is available.
func (r *Ring) WaitSQ() { <-r.sqCh }

// WaitCQ blocks until at least one CQE is available.
func (r *Ring) WaitCQ() { <-r.cqCh }

// SubmitAndWait is the blocking-bridge helper: it submits d and blocks until
// the service posts a CQE whose Cookie matches d.Cookie.
// It is intended for single-flight callers that want to keep existing
// synchronous call semantics while routing through the SQ/CQ path.
func (r *Ring) SubmitAndWait(d Descriptor) (Descriptor, error) {
	if err := r.Submit(d); err != nil {
		return Descriptor{}, err
	}
	for {
		r.WaitCQ()
		cqe, err := r.PollCQ()
		if err != nil {
			// Spurious wakeup — keep waiting.
			continue
		}
		if cqe.Cookie == d.Cookie {
			return cqe, nil
		}
		// CQE belongs to a different in-flight request (should not happen for
		// single-threaded domains, but handle it gracefully by re-queueing).
		// We just consumed a slot from the CQ so there is always room to put
		// it back; the error is therefore safe to ignore here.
		_ = r.Complete(cqe)
	}
}
