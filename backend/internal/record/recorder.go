package record

import "fmt"

// Recorder collects per-system RecordBuckets within a stack of named
// namespaces. It is intended for use during a single game logic execution
// cycle (e.g. one event settlement), not as a long-lived object.
//
// Usage:
//
//	rec := record.NewRecorder(reg)
//	rec.PushNamespace("event_execution")
//	    invB := rec.Bucket("inventory").(*inventory.Bucket)
//	    invB.AddChange(1, 0, 5, 0)
//	rec.PopNamespace()
//	diff, _ := reg.BuildDiff(rec)
type Recorder struct {
	reg       *Registry
	stack     []*namespace
	completed []*namespace
}

type namespace struct {
	name    string
	buckets map[string]RecordBucket // systemName -> bucket (created lazily)
}

// NewRecorder creates a Recorder with no active namespace.
// Callers must PushNamespace before retrieving buckets.
func NewRecorder(reg *Registry) *Recorder {
	return &Recorder{reg: reg}
}

// PushNamespace starts a new named namespace. Buckets retrieved after this
// call belong to this namespace until PopNamespace is called.
func (r *Recorder) PushNamespace(name string) {
	r.stack = append(r.stack, &namespace{
		name:    name,
		buckets: make(map[string]RecordBucket),
	})
}

// PopNamespace closes the current namespace. Panics if none is active.
func (r *Recorder) PopNamespace() {
	if len(r.stack) == 0 {
		panic("record: PopNamespace called with no active namespace")
	}
	n := len(r.stack) - 1
	r.completed = append(r.completed, r.stack[n])
	r.stack = r.stack[:n]
}

// Bucket returns the RecordBucket for the given system within the current
// namespace. If no bucket exists yet, one is created via the Registry.
// Returns nil if no namespace is active or the system is not registered.
func (r *Recorder) Bucket(systemName string) RecordBucket {
	if len(r.stack) == 0 {
		return nil
	}

	ns := r.stack[len(r.stack)-1]
	b, ok := ns.buckets[systemName]
	if !ok {
		if r.reg == nil {
			return nil
		}
		p := r.reg.Provider(systemName)
		if p == nil {
			return nil
		}
		b = p.NewBucket()
		ns.buckets[systemName] = b
	}
	return b
}

// PopAll drains all namespaces (popping any still-open ones first) and returns
// them in FIFO order of popping (innermost first, outermost last).
func (r *Recorder) PopAll() []*namespace {
	// Drain remaining open namespaces from inner to outer.
	for i := len(r.stack) - 1; i >= 0; i-- {
		r.completed = append(r.completed, r.stack[i])
	}
	r.stack = nil

	out := r.completed
	r.completed = nil
	return out
}

// Active reports whether there is an active namespace.
func (r *Recorder) Active() bool {
	return len(r.stack) > 0
}

// Get is a generic helper that retrieves a typed bucket from the Recorder.
// The system name is derived from T.SystemName(), so the type itself determines
// which bucket to fetch — no string argument needed.
//
//	invB := record.Get[*inventory.Bucket](rec)
//
// T.SystemName() must be callable on a nil pointer (i.e. it must not access
// the receiver), which is the natural case for a constant return.
//
// Get panics if the bucket exists but is not of type T (programmer error).
// Returns zero value if no bucket exists or no namespace is active.
func Get[T RecordBucket](rec *Recorder) T {
	var zero T
	name := zero.SystemName()

	b := rec.Bucket(name)
	if b == nil {
		return zero
	}
	typed, ok := b.(T)
	if !ok {
		panic("record.Get: bucket type mismatch for system " + name)
	}
	return typed
}

// Recordable is implemented by game state types that accept a Recorder
// for the current execution cycle.
type Recordable interface {
	SetRecorder(rec *Recorder)
	ClearRecorder()
}

// String returns a debug representation of the current stack.
func (r *Recorder) String() string {
	if len(r.stack) == 0 {
		return "recorder: (empty)"
	}
	s := "recorder:"
	for i, ns := range r.stack {
		s += fmt.Sprintf("\n  [%d] %s (%d buckets)", i, ns.name, len(ns.buckets))
	}
	return s
}
