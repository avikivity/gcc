// Copyright 2009 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package runtime

import (
	"unsafe"
)

// defined constants
const (
	// G status
	//
	// Beyond indicating the general state of a G, the G status
	// acts like a lock on the goroutine's stack (and hence its
	// ability to execute user code).
	//
	// If you add to this list, add to the list
	// of "okay during garbage collection" status
	// in mgcmark.go too.

	// _Gidle means this goroutine was just allocated and has not
	// yet been initialized.
	_Gidle = iota // 0

	// _Grunnable means this goroutine is on a run queue. It is
	// not currently executing user code. The stack is not owned.
	_Grunnable // 1

	// _Grunning means this goroutine may execute user code. The
	// stack is owned by this goroutine. It is not on a run queue.
	// It is assigned an M and a P.
	_Grunning // 2

	// _Gsyscall means this goroutine is executing a system call.
	// It is not executing user code. The stack is owned by this
	// goroutine. It is not on a run queue. It is assigned an M.
	_Gsyscall // 3

	// _Gwaiting means this goroutine is blocked in the runtime.
	// It is not executing user code. It is not on a run queue,
	// but should be recorded somewhere (e.g., a channel wait
	// queue) so it can be ready()d when necessary. The stack is
	// not owned *except* that a channel operation may read or
	// write parts of the stack under the appropriate channel
	// lock. Otherwise, it is not safe to access the stack after a
	// goroutine enters _Gwaiting (e.g., it may get moved).
	_Gwaiting // 4

	// _Gmoribund_unused is currently unused, but hardcoded in gdb
	// scripts.
	_Gmoribund_unused // 5

	// _Gdead means this goroutine is currently unused. It may be
	// just exited, on a free list, or just being initialized. It
	// is not executing user code. It may or may not have a stack
	// allocated. The G and its stack (if any) are owned by the M
	// that is exiting the G or that obtained the G from the free
	// list.
	_Gdead // 6

	// _Genqueue_unused is currently unused.
	_Genqueue_unused // 7

	// _Gcopystack means this goroutine's stack is being moved. It
	// is not executing user code and is not on a run queue. The
	// stack is owned by the goroutine that put it in _Gcopystack.
	_Gcopystack // 8

	// _Gscan combined with one of the above states other than
	// _Grunning indicates that GC is scanning the stack. The
	// goroutine is not executing user code and the stack is owned
	// by the goroutine that set the _Gscan bit.
	//
	// _Gscanrunning is different: it is used to briefly block
	// state transitions while GC signals the G to scan its own
	// stack. This is otherwise like _Grunning.
	//
	// atomicstatus&~Gscan gives the state the goroutine will
	// return to when the scan completes.
	_Gscan         = 0x1000
	_Gscanrunnable = _Gscan + _Grunnable // 0x1001
	_Gscanrunning  = _Gscan + _Grunning  // 0x1002
	_Gscansyscall  = _Gscan + _Gsyscall  // 0x1003
	_Gscanwaiting  = _Gscan + _Gwaiting  // 0x1004
)

const (
	// P status
	_Pidle    = iota
	_Prunning // Only this P is allowed to change from _Prunning.
	_Psyscall
	_Pgcstop
	_Pdead
)

// Mutual exclusion locks.  In the uncontended case,
// as fast as spin locks (just a few user-level instructions),
// but on the contention path they sleep in the kernel.
// A zeroed Mutex is unlocked (no need to initialize each lock).
type mutex struct {
	// Futex-based impl treats it as uint32 key,
	// while sema-based impl as M* waitm.
	// Used to be a union, but unions break precise GC.
	key uintptr
}

// sleep and wakeup on one-time events.
// before any calls to notesleep or notewakeup,
// must call noteclear to initialize the Note.
// then, exactly one thread can call notesleep
// and exactly one thread can call notewakeup (once).
// once notewakeup has been called, the notesleep
// will return.  future notesleep will return immediately.
// subsequent noteclear must be called only after
// previous notesleep has returned, e.g. it's disallowed
// to call noteclear straight after notewakeup.
//
// notetsleep is like notesleep but wakes up after
// a given number of nanoseconds even if the event
// has not yet happened.  if a goroutine uses notetsleep to
// wake up early, it must wait to call noteclear until it
// can be sure that no other goroutine is calling
// notewakeup.
//
// notesleep/notetsleep are generally called on g0,
// notetsleepg is similar to notetsleep but is called on user g.
type note struct {
	// Futex-based impl treats it as uint32 key,
	// while sema-based impl as M* waitm.
	// Used to be a union, but unions break precise GC.
	key uintptr
}

type funcval struct {
	fn uintptr
	// variable-size, fn-specific data here
}

type iface struct {
	tab  unsafe.Pointer
	data unsafe.Pointer
}

type eface struct {
	_type *_type
	data  unsafe.Pointer
}

func efaceOf(ep *interface{}) *eface {
	return (*eface)(unsafe.Pointer(ep))
}

// The guintptr, muintptr, and puintptr are all used to bypass write barriers.
// It is particularly important to avoid write barriers when the current P has
// been released, because the GC thinks the world is stopped, and an
// unexpected write barrier would not be synchronized with the GC,
// which can lead to a half-executed write barrier that has marked the object
// but not queued it. If the GC skips the object and completes before the
// queuing can occur, it will incorrectly free the object.
//
// We tried using special assignment functions invoked only when not
// holding a running P, but then some updates to a particular memory
// word went through write barriers and some did not. This breaks the
// write barrier shadow checking mode, and it is also scary: better to have
// a word that is completely ignored by the GC than to have one for which
// only a few updates are ignored.
//
// Gs, Ms, and Ps are always reachable via true pointers in the
// allgs, allm, and allp lists or (during allocation before they reach those lists)
// from stack variables.

// A guintptr holds a goroutine pointer, but typed as a uintptr
// to bypass write barriers. It is used in the Gobuf goroutine state
// and in scheduling lists that are manipulated without a P.
//
// The Gobuf.g goroutine pointer is almost always updated by assembly code.
// In one of the few places it is updated by Go code - func save - it must be
// treated as a uintptr to avoid a write barrier being emitted at a bad time.
// Instead of figuring out how to emit the write barriers missing in the
// assembly manipulation, we change the type of the field to uintptr,
// so that it does not require write barriers at all.
//
// Goroutine structs are published in the allg list and never freed.
// That will keep the goroutine structs from being collected.
// There is never a time that Gobuf.g's contain the only references
// to a goroutine: the publishing of the goroutine in allg comes first.
// Goroutine pointers are also kept in non-GC-visible places like TLS,
// so I can't see them ever moving. If we did want to start moving data
// in the GC, we'd need to allocate the goroutine structs from an
// alternate arena. Using guintptr doesn't make that problem any worse.
type guintptr uintptr

//go:nosplit
func (gp guintptr) ptr() *g { return (*g)(unsafe.Pointer(gp)) }

//go:nosplit
func (gp *guintptr) set(g *g) { *gp = guintptr(unsafe.Pointer(g)) }

/*
//go:nosplit
func (gp *guintptr) cas(old, new guintptr) bool {
	return atomic.Casuintptr((*uintptr)(unsafe.Pointer(gp)), uintptr(old), uintptr(new))
}
*/

type puintptr uintptr

//go:nosplit
func (pp puintptr) ptr() *p { return (*p)(unsafe.Pointer(pp)) }

//go:nosplit
func (pp *puintptr) set(p *p) { *pp = puintptr(unsafe.Pointer(p)) }

type muintptr uintptr

//go:nosplit
func (mp muintptr) ptr() *m { return (*m)(unsafe.Pointer(mp)) }

//go:nosplit
func (mp *muintptr) set(m *m) { *mp = muintptr(unsafe.Pointer(m)) }

// sudog represents a g in a wait list, such as for sending/receiving
// on a channel.
//
// sudog is necessary because the g ↔ synchronization object relation
// is many-to-many. A g can be on many wait lists, so there may be
// many sudogs for one g; and many gs may be waiting on the same
// synchronization object, so there may be many sudogs for one object.
//
// sudogs are allocated from a special pool. Use acquireSudog and
// releaseSudog to allocate and free them.
/*
Commented out for gccgo for now.

type sudog struct {
	// The following fields are protected by the hchan.lock of the
	// channel this sudog is blocking on. shrinkstack depends on
	// this.

	g          *g
	selectdone *uint32 // CAS to 1 to win select race (may point to stack)
	next       *sudog
	prev       *sudog
	elem       unsafe.Pointer // data element (may point to stack)

	// The following fields are never accessed concurrently.
	// waitlink is only accessed by g.

	releasetime int64
	ticket      uint32
	waitlink    *sudog // g.waiting list
	c           *hchan // channel
}
*/

type gcstats struct {
	// the struct must consist of only uint64's,
	// because it is casted to uint64[].
	nhandoff    uint64
	nhandoffcnt uint64
	nprocyield  uint64
	nosyield    uint64
	nsleep      uint64
}

/*
Not used by gccgo.

type libcall struct {
	fn   uintptr
	n    uintptr // number of parameters
	args uintptr // parameters
	r1   uintptr // return values
	r2   uintptr
	err  uintptr // error number
}

*/

/*
Not used by gccgo.

// describes how to handle callback
type wincallbackcontext struct {
	gobody       unsafe.Pointer // go function to call
	argsize      uintptr        // callback arguments size (in bytes)
	restorestack uintptr        // adjust stack on return by (in bytes) (386 only)
	cleanstack   bool
}
*/

/*
Not used by gccgo.

// Stack describes a Go execution stack.
// The bounds of the stack are exactly [lo, hi),
// with no implicit data structures on either side.
type stack struct {
	lo uintptr
	hi uintptr
}

// stkbar records the state of a G's stack barrier.
type stkbar struct {
	savedLRPtr uintptr // location overwritten by stack barrier PC
	savedLRVal uintptr // value overwritten at savedLRPtr
}
*/

type g struct {
	// Stack parameters.
	// stack describes the actual stack memory: [stack.lo, stack.hi).
	// stackguard0 is the stack pointer compared in the Go stack growth prologue.
	// It is stack.lo+StackGuard normally, but can be StackPreempt to trigger a preemption.
	// stackguard1 is the stack pointer compared in the C stack growth prologue.
	// It is stack.lo+StackGuard on g0 and gsignal stacks.
	// It is ~0 on other goroutine stacks, to trigger a call to morestackc (and crash).
	// Not for gccgo: stack       stack   // offset known to runtime/cgo
	// Not for gccgo: stackguard0 uintptr // offset known to liblink
	// Not for gccgo: stackguard1 uintptr // offset known to liblink

	_panic *_panic // innermost panic - offset known to liblink
	_defer *_defer // innermost defer
	m      *m      // current m; offset known to arm liblink
	// Not for gccgo: stackAlloc     uintptr // stack allocation is [stack.lo,stack.lo+stackAlloc)
	// Not for gccgo: sched          gobuf
	// Not for gccgo: syscallsp      uintptr        // if status==Gsyscall, syscallsp = sched.sp to use during gc
	// Not for gccgo: syscallpc      uintptr        // if status==Gsyscall, syscallpc = sched.pc to use during gc
	// Not for gccgo: stkbar         []stkbar       // stack barriers, from low to high (see top of mstkbar.go)
	// Not for gccgo: stkbarPos      uintptr        // index of lowest stack barrier not hit
	// Not for gccgo: stktopsp       uintptr        // expected sp at top of stack, to check in traceback
	param        unsafe.Pointer // passed parameter on wakeup
	atomicstatus uint32
	// Not for gccgo: stackLock      uint32 // sigprof/scang lock; TODO: fold in to atomicstatus
	goid           int64
	waitsince      int64  // approx time when the g become blocked
	waitreason     string // if status==Gwaiting
	schedlink      guintptr
	preempt        bool     // preemption signal, duplicates stackguard0 = stackpreempt
	paniconfault   bool     // panic (instead of crash) on unexpected fault address
	preemptscan    bool     // preempted g does scan for gc
	gcscandone     bool     // g has scanned stack; protected by _Gscan bit in status
	gcscanvalid    bool     // false at start of gc cycle, true if G has not run since last scan; transition from true to false by calling queueRescan and false to true by calling dequeueRescan
	throwsplit     bool     // must not split stack
	raceignore     int8     // ignore race detection events
	sysblocktraced bool     // StartTrace has emitted EvGoInSyscall about this goroutine
	sysexitticks   int64    // cputicks when syscall has returned (for tracing)
	traceseq       uint64   // trace event sequencer
	tracelastp     puintptr // last P emitted an event for this goroutine
	lockedm        *m
	sig            uint32

	// Temporary gccgo field.
	writenbuf int32
	// Not for gccgo yet: writebuf       []byte
	// Temporary different type for gccgo.
	writebuf *byte

	sigcode0 uintptr
	sigcode1 uintptr
	sigpc    uintptr
	gopc     uintptr // pc of go statement that created this goroutine
	startpc  uintptr // pc of goroutine function
	racectx  uintptr
	// Not for gccgo for now: waiting        *sudog    // sudog structures this g is waiting on (that have a valid elem ptr); in lock order
	// Not for gccgo: cgoCtxt        []uintptr // cgo traceback context

	// Per-G GC state

	// gcRescan is this G's index in work.rescan.list. If this is
	// -1, this G is not on the rescan list.
	//
	// If gcphase != _GCoff and this G is visible to the garbage
	// collector, writes to this are protected by work.rescan.lock.
	gcRescan int32

	// gcAssistBytes is this G's GC assist credit in terms of
	// bytes allocated. If this is positive, then the G has credit
	// to allocate gcAssistBytes bytes without assisting. If this
	// is negative, then the G must correct this by performing
	// scan work. We track this in bytes to make it fast to update
	// and check for debt in the malloc hot path. The assist ratio
	// determines how this corresponds to scan work debt.
	gcAssistBytes int64

	// Remaining fields are specific to gccgo.

	exception unsafe.Pointer // current exception being thrown
	isforeign bool           // whether current exception is not from Go

	// Fields that hold stack and context information if status is Gsyscall
	gcstack       unsafe.Pointer
	gcstacksize   uintptr
	gcnextsegment unsafe.Pointer
	gcnextsp      unsafe.Pointer
	gcinitialsp   unsafe.Pointer
	gcregs        g_ucontext_t

	entry    unsafe.Pointer // goroutine entry point
	fromgogo bool           // whether entered from gogo function

	issystem     bool // do not output in stack dump
	isbackground bool // ignore in deadlock detector

	traceback *traceback // stack traceback buffer

	context      g_ucontext_t       // saved context for setcontext
	stackcontext [10]unsafe.Pointer // split-stack context
}

type m struct {
	g0 *g // goroutine with scheduling stack
	// Not for gccgo: morebuf gobuf  // gobuf arg to morestack
	// Not for gccgo: divmod  uint32 // div/mod denominator for arm - known to liblink

	// Fields not known to debuggers.
	procid  uint64 // for debuggers, but offset not hard-coded
	gsignal *g     // signal-handling g
	sigmask sigset // storage for saved signal mask
	// Not for gccgo: tls           [6]uintptr // thread-local storage (for x86 extern register)
	mstartfn    uintptr
	curg        *g       // current running goroutine
	caughtsig   guintptr // goroutine running during fatal signal
	p           puintptr // attached p for executing go code (nil if not executing go code)
	nextp       puintptr
	id          int32
	mallocing   int32
	throwing    int32
	preemptoff  string // if != "", keep curg running on this m
	locks       int32
	softfloat   int32
	dying       int32
	profilehz   int32
	helpgc      int32
	spinning    bool // m is out of work and is actively looking for work
	blocked     bool // m is blocked on a note
	inwb        bool // m is executing a write barrier
	newSigstack bool // minit on C thread called sigaltstack
	printlock   int8
	fastrand    uint32
	ncgocall    uint64 // number of cgo calls in total
	ncgo        int32  // number of cgo calls currently in progress
	// Not for gccgo: cgoCallersUse uint32      // if non-zero, cgoCallers in use temporarily
	// Not for gccgo: cgoCallers    *cgoCallers // cgo traceback if crashing in cgo call
	park        note
	alllink     *m // on allm
	schedlink   muintptr
	mcache      *mcache
	lockedg     *g
	createstack [32]location // stack that created this thread.
	// Not for gccgo: freglo        [16]uint32  // d[i] lsb and f[i]
	// Not for gccgo: freghi        [16]uint32  // d[i] msb and f[i+16]
	// Not for gccgo: fflag         uint32      // floating point compare flags
	locked        uint32  // tracking for lockosthread
	nextwaitm     uintptr // next m waiting for lock
	gcstats       gcstats
	needextram    bool
	traceback     uint8
	waitunlockf   unsafe.Pointer // todo go func(*g, unsafe.pointer) bool
	waitlock      unsafe.Pointer
	waittraceev   byte
	waittraceskip int
	startingtrace bool
	syscalltick   uint32
	// Not for gccgo: thread        uintptr // thread handle

	// these are here because they are too large to be on the stack
	// of low-level NOSPLIT functions.
	// Not for gccgo: libcall   libcall
	// Not for gccgo: libcallpc uintptr // for cpu profiler
	// Not for gccgo: libcallsp uintptr
	// Not for gccgo: libcallg  guintptr
	// Not for gccgo: syscall   libcall // stores syscall parameters on windows

	mos mOS

	// Remaining fields are specific to gccgo.

	gsignalstack     unsafe.Pointer // stack for gsignal
	gsignalstacksize uintptr

	dropextram bool // drop after call is done

	gcing int32

	cgomal *cgoMal // allocations via _cgo_allocate
}

type p struct {
	lock mutex

	id          int32
	status      uint32 // one of pidle/prunning/...
	link        puintptr
	schedtick   uint32   // incremented on every scheduler call
	syscalltick uint32   // incremented on every system call
	m           muintptr // back-link to associated m (nil if idle)
	mcache      *mcache
	// Not for gccgo: racectx     uintptr

	// Not for gccgo yet: deferpool    [5][]*_defer // pool of available defer structs of different sizes (see panic.go)
	// Not for gccgo yet: deferpoolbuf [5][32]*_defer
	// Temporary gccgo type for deferpool field.
	deferpool *_defer

	// Cache of goroutine ids, amortizes accesses to runtime·sched.goidgen.
	goidcache    uint64
	goidcacheend uint64

	// Queue of runnable goroutines. Accessed without lock.
	runqhead uint32
	runqtail uint32
	runq     [256]guintptr
	// runnext, if non-nil, is a runnable G that was ready'd by
	// the current G and should be run next instead of what's in
	// runq if there's time remaining in the running G's time
	// slice. It will inherit the time left in the current time
	// slice. If a set of goroutines is locked in a
	// communicate-and-wait pattern, this schedules that set as a
	// unit and eliminates the (potentially large) scheduling
	// latency that otherwise arises from adding the ready'd
	// goroutines to the end of the run queue.
	runnext guintptr

	// Available G's (status == Gdead)
	gfree    *g
	gfreecnt int32

	// Not for gccgo for now: sudogcache []*sudog
	// Not for gccgo for now: sudogbuf   [128]*sudog

	// Not for gccgo for now: tracebuf traceBufPtr

	// Not for gccgo for now: palloc persistentAlloc // per-P to avoid mutex

	// Per-P GC state
	// Not for gccgo for now: gcAssistTime     int64 // Nanoseconds in assistAlloc
	// Not for gccgo for now: gcBgMarkWorker   guintptr
	// Not for gccgo for now: gcMarkWorkerMode gcMarkWorkerMode

	// gcw is this P's GC work buffer cache. The work buffer is
	// filled by write barriers, drained by mutator assists, and
	// disposed on certain GC state transitions.
	// Not for gccgo for now: gcw gcWork

	runSafePointFn uint32 // if 1, run sched.safePointFn at next safe point

	pad [64]byte
}

const (
	// The max value of GOMAXPROCS.
	// There are no fundamental restrictions on the value.
	_MaxGomaxprocs = 1 << 8
)

/*
Commented out for gccgo for now.

type schedt struct {
	// accessed atomically. keep at top to ensure alignment on 32-bit systems.
	goidgen  uint64
	lastpoll uint64

	lock mutex

	midle        muintptr // idle m's waiting for work
	nmidle       int32    // number of idle m's waiting for work
	nmidlelocked int32    // number of locked m's waiting for work
	mcount       int32    // number of m's that have been created
	maxmcount    int32    // maximum number of m's allowed (or die)

	ngsys uint32 // number of system goroutines; updated atomically

	pidle      puintptr // idle p's
	npidle     uint32
	nmspinning uint32 // See "Worker thread parking/unparking" comment in proc.go.

	// Global runnable queue.
	runqhead guintptr
	runqtail guintptr
	runqsize int32

	// Global cache of dead G's.
	gflock       mutex
	gfreeStack   *g
	gfreeNoStack *g
	ngfree       int32

	// Central cache of sudog structs.
	sudoglock  mutex
	sudogcache *sudog

	// Central pool of available defer structs of different sizes.
	deferlock mutex
	deferpool [5]*_defer

	gcwaiting  uint32 // gc is waiting to run
	stopwait   int32
	stopnote   note
	sysmonwait uint32
	sysmonnote note

	// safepointFn should be called on each P at the next GC
	// safepoint if p.runSafePointFn is set.
	safePointFn   func(*p)
	safePointWait int32
	safePointNote note

	profilehz int32 // cpu profiling rate

	procresizetime int64 // nanotime() of last change to gomaxprocs
	totaltime      int64 // ∫gomaxprocs dt up to procresizetime
}
*/

// The m.locked word holds two pieces of state counting active calls to LockOSThread/lockOSThread.
// The low bit (LockExternal) is a boolean reporting whether any LockOSThread call is active.
// External locks are not recursive; a second lock is silently ignored.
// The upper bits of m.locked record the nesting depth of calls to lockOSThread
// (counting up by LockInternal), popped by unlockOSThread (counting down by LockInternal).
// Internal locks can be recursive. For instance, a lock for cgo can occur while the main
// goroutine is holding the lock during the initialization phase.
const (
	_LockExternal = 1
	_LockInternal = 2
)

const (
	_SigNotify   = 1 << iota // let signal.Notify have signal, even if from kernel
	_SigKill                 // if signal.Notify doesn't take it, exit quietly
	_SigThrow                // if signal.Notify doesn't take it, exit loudly
	_SigPanic                // if the signal is from the kernel, panic
	_SigDefault              // if the signal isn't explicitly requested, don't monitor it
	_SigHandling             // our signal handler is registered
	_SigGoExit               // cause all runtime procs to exit (only used on Plan 9).
	_SigSetStack             // add SA_ONSTACK to libc handler
	_SigUnblock              // unblocked in minit
)

/*
gccgo does not use this.

// Layout of in-memory per-function information prepared by linker
// See https://golang.org/s/go12symtab.
// Keep in sync with linker
// and with package debug/gosym and with symtab.go in package runtime.
type _func struct {
	entry   uintptr // start pc
	nameoff int32   // function name

	args int32 // in/out args size
	_    int32 // previously legacy frame size; kept for layout compatibility

	pcsp      int32
	pcfile    int32
	pcln      int32
	npcdata   int32
	nfuncdata int32
}

*/

// Lock-free stack node.
// // Also known to export_test.go.
type lfnode struct {
	next    uint64
	pushcnt uintptr
}

type forcegcstate struct {
	lock mutex
	g    *g
	idle uint32
}

// startup_random_data holds random bytes initialized at startup. These come from
// the ELF AT_RANDOM auxiliary vector (vdso_linux_amd64.go or os_linux_386.go).
var startupRandomData []byte

/*
// extendRandom extends the random numbers in r[:n] to the whole slice r.
// Treats n<0 as n==0.
func extendRandom(r []byte, n int) {
	if n < 0 {
		n = 0
	}
	for n < len(r) {
		// Extend random bits using hash function & time seed
		w := n
		if w > 16 {
			w = 16
		}
		h := memhash(unsafe.Pointer(&r[n-w]), uintptr(nanotime()), uintptr(w))
		for i := 0; i < sys.PtrSize && n < len(r); i++ {
			r[n] = byte(h)
			n++
			h >>= 8
		}
	}
}
*/

// deferred subroutine calls
// This is the gccgo version.
type _defer struct {
	// The next entry in the stack.
	next *_defer

	// The stack variable for the function which called this defer
	// statement.  This is set to true if we are returning from
	// that function, false if we are panicing through it.
	frame *bool

	// The value of the panic stack when this function is
	// deferred.  This function can not recover this value from
	// the panic stack.  This can happen if a deferred function
	// has a defer statement itself.
	_panic *_panic

	// The function to call.
	pfn uintptr

	// The argument to pass to the function.
	arg unsafe.Pointer

	// The return address that a recover thunk matches against.
	// This is set by __go_set_defer_retaddr which is called by
	// the thunks created by defer statements.
	retaddr uintptr

	// Set to true if a function created by reflect.MakeFunc is
	// permitted to recover.  The return address of such a
	// function function will be somewhere in libffi, so __retaddr
	// is not useful.
	makefunccanrecover bool

	// Set to true if this defer stack entry is not part of the
	// defer pool.
	special bool
}

// panics
// This is the gccgo version.
type _panic struct {
	// The next entry in the stack.
	next *_panic

	// The value associated with this panic.
	arg interface{}

	// Whether this panic has been recovered.
	recovered bool

	// Whether this panic was pushed on the stack because of an
	// exception thrown in some other language.
	isforeign bool
}

const (
	_TraceRuntimeFrames = 1 << iota // include frames for internal runtime functions.
	_TraceTrap                      // the initial PC, SP are from a trap, not a return PC from a call
	_TraceJumpStack                 // if traceback is on a systemstack, resume trace at g that called into it
)

// The maximum number of frames we print for a traceback
const _TracebackMaxFrames = 100

var (
	//	emptystring string
	//	allglen     uintptr
	//	allm        *m
	//	allp        [_MaxGomaxprocs + 1]*p
	//	gomaxprocs  int32
	//	panicking   uint32

	ncpu int32

//	forcegc     forcegcstate
//	sched       schedt
//	newprocs    int32

// Information about what cpu features are available.
// Set on startup in asm_{x86,amd64}.s.
//	cpuid_ecx         uint32
//	cpuid_edx         uint32
//	cpuid_ebx7        uint32
//	lfenceBeforeRdtsc bool
//	support_avx       bool
//	support_avx2      bool

//	goarm                uint8 // set by cmd/link on arm systems
//	framepointer_enabled bool  // set by cmd/link
)

// Set by the linker so the runtime can determine the buildmode.
var (
	islibrary bool // -buildmode=c-shared
	isarchive bool // -buildmode=c-archive
)

// Types that are only used by gccgo.

// g_ucontext_t is a Go version of the C ucontext_t type, used by getcontext.
// _sizeof_ucontext_t is defined by mkrsysinfo.sh from <ucontext.h>.
// On some systems getcontext and friends require a value that is
// aligned to a 16-byte boundary.  We implement this by increasing the
// required size and picking an appropriate offset when we use the
// array.
type g_ucontext_t [(_sizeof_ucontext_t + 15) / unsafe.Sizeof(unsafe.Pointer(nil))]unsafe.Pointer

// traceback is used to collect stack traces from other goroutines.
type traceback struct {
	gp     *g
	locbuf [_TracebackMaxFrames]location
	c      int
}

// location is a location in the program, used for backtraces.
type location struct {
	pc       uintptr
	filename string
	function string
	lineno   int
}

// cgoMal tracks allocations made by _cgo_allocate
// FIXME: _cgo_allocate has been removed from gc and can probably be
// removed from gccgo too.
type cgoMal struct {
	next  *cgoMal
	alloc unsafe.Pointer
}

// sigset is the Go version of the C type sigset_t.
// _sigset_t is defined by the Makefile from <signal.h>.
type sigset _sigset_t
