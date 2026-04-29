//go:build tinygo

package runtime

import (
    "unsafe"
    "kernel"
)

// Process state (one per domain)
type process struct {
    id         kernel.DomainID
    syscallQ   unsafe.Pointer
    replyQ     unsafe.Pointer
    heapStart  uintptr
    heapSize   uint32
    caps       map[capHandle]kernel.CapabilityID
}

// Capability handle (process-local)
type capHandle uint32

var currentProcess *process
var nextCapHandle capHandle = 1

// Initialize domain runtime
//export runtimeDomainInit
func runtimeDomainInit(
    domainID kernel.DomainID,
    syscallQ, replyQ unsafe.Pointer,
    heapStart uintptr,
    heapSize uint32,
) {
    // Initialize process
    currentProcess = &process{
        id:        domainID,
        syscallQ:  syscallQ,
        replyQ:    replyQ,
        heapStart: heapStart,
        heapSize:  heapSize,
        caps:      make(map[capHandle]kernel.CapabilityID),
    }

    // Initialize GC heap
    gcInit(heapStart, heapSize)

    // Initialize goroutine system
    initGoroutines()

    // Call user main
    go mainMain()

    // Start scheduler
    schedule()
}

// Goroutine structure (simplified TinyGo version)
type g struct {
    stack    stack
    sched    gobuf
    goid     int64
    state    uint32
    alllink  *g      // All goroutines
    schedlink *g     // Runnable goroutines
}

type stack struct {
    lo uintptr
    hi uintptr
}

type gobuf struct {
    sp uintptr
    pc uintptr
    lr uintptr  // ARM link register
}

// Goroutine states
const (
    _Gidle uint32 = iota
    _Grunnable
    _Grunning
    _Gwaiting
    _Gdead
)

var allgs *g        // All goroutines
var runqhead *g     // Run queue head
var runqtail *g     // Run queue tail
var currentg *g     // Current goroutine
var goidgen int64   // Goroutine ID generator

// Initialize goroutine system
func initGoroutines() {
    allgs = nil
    runqhead = nil
    runqtail = nil
    goidgen = 1

    // Create main goroutine
    maing := newg(8192) // 8KB stack
    maing.state = _Grunning
    currentg = maing
}

// Create new goroutine
func newg(stackSize uintptr) *g {
    // Allocate stack
    stackMem := runtimeAlloc(stackSize)
    if stackMem == nil {
        return nil
    }

    // Allocate g structure
    gp := (*g)(runtimeAlloc(unsafe.Sizeof(g{})))
    if gp == nil {
        return nil
    }

    gp.stack.lo = uintptr(stackMem)
    gp.stack.hi = uintptr(stackMem) + stackSize
    gp.goid = goidgen
    gp.state = _Gidle
    goidgen++

    // Add to all goroutines list
    gp.alllink = allgs
    allgs = gp

    return gp
}

// Go statement implementation
//export runtime.newproc
func newproc(fn unsafe.Pointer, argSize uintptr) {
    // Create new goroutine
    newg := newg(8192)
    if newg == nil {
        panic("cannot create goroutine")
    }

    // Setup initial stack frame
    // (simplified - real implementation would copy args)
    sp := newg.stack.hi - argSize - 16
    newg.sched.sp = sp
    newg.sched.pc = uintptr(fn)

    // Add to run queue
    newg.state = _Grunnable
    runqput(newg)
}

// Add goroutine to run queue
func runqput(gp *g) {
    gp.schedlink = nil

    if runqtail == nil {
        runqhead = gp
        runqtail = gp
    } else {
        runqtail.schedlink = gp
        runqtail = gp
    }
}

// Get goroutine from run queue
func runqget() *g {
    gp := runqhead
    if gp != nil {
        runqhead = gp.schedlink
        if runqhead == nil {
            runqtail = nil
        }
        gp.schedlink = nil
    }
    return gp
}

// Cooperative scheduler
func schedule() {
    for {
        // Get next runnable goroutine
        gp := runqget()

        if gp == nil {
            // No work - idle
            // Could yield to FreeRTOS or wait
            vTaskDelay(1)
            continue
        }

        // Switch to this goroutine
        gp.state = _Grunning
        currentg = gp

        // Execute (this would be assembly in real TinyGo)
        gogo(&gp.sched)

        // When it yields back, handle state
        if gp.state == _Grunnable {
            runqput(gp)
        } else if gp.state == _Gdead {
            // Goroutine exited - GC will collect it
        }
    }
}

// Yield execution (called from goroutine)
//export runtime.Gosched
func Gosched() {
    // Save current state
    mcall(gosched_m)
}

// Channel implementation hooks into FreeRTOS queues
// (see runtime/chan.go for full implementation)

//export mainMain
func mainMain() {
    // This calls the user's main.main()
    // Defined in user code
}
