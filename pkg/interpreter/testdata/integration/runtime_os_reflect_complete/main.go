// runtime_os_reflect_complete exercises additions from v0.90.0:
// runtime: (*Frames).Next, SetDefaultGOMAXPROCS (Go 1.25+),
//          BlockProfile, GoroutineProfile, MemProfile, MutexProfile,
//          StartTrace, StopTrace, ReadTrace;
// os:      OpenInRoot (Go 1.25+), (*File).WriteTo,
//          (*File).SetDeadline, SetReadDeadline, SetWriteDeadline;
// net/http: (*Server).RegisterOnShutdown, (*Server).SetKeepAlivesEnabled;
// text/template: (*Template).AddParseTree, (*Template).DefinedTemplates;
// reflect: TypeFor[T] (Go 1.22+).
// Expected: 0 violations.
package main

import (
	"net/http"
	"os"
	"reflect"
	"runtime"
	"text/template"
	"time"
)

func main() {
	// runtime: (*Frames).Next.
	var pcs [1]uintptr
	n := runtime.Callers(0, pcs[:])
	frames := runtime.CallersFrames(pcs[:n])
	frame, more := frames.Next()
	_, _ = frame, more

	// runtime: SetDefaultGOMAXPROCS (Go 1.25+).
	runtime.SetDefaultGOMAXPROCS()

	// runtime: profiling functions.
	n1, ok1 := runtime.BlockProfile(nil)
	_, _ = n1, ok1

	n2, ok2 := runtime.GoroutineProfile(nil)
	_, _ = n2, ok2

	n3, ok3 := runtime.MemProfile(nil, false)
	_, _ = n3, ok3

	n4, ok4 := runtime.MutexProfile(nil)
	_, _ = n4, ok4

	// runtime: tracing functions.
	_ = runtime.StartTrace()
	runtime.StopTrace()
	_ = runtime.ReadTrace()

	// os: OpenInRoot (Go 1.25+).
	f2, err := os.OpenInRoot("/tmp", "nonexistent")
	_, _ = f2, err

	// os: (*File).WriteTo.
	f := os.Stdin
	_, _ = f.WriteTo(os.Stdout)

	// os: (*File).SetDeadline/SetReadDeadline/SetWriteDeadline.
	t0 := time.Time{}
	_ = f.SetDeadline(t0)
	_ = f.SetReadDeadline(t0)
	_ = f.SetWriteDeadline(t0)

	// net/http: (*Server).RegisterOnShutdown and SetKeepAlivesEnabled.
	srv := &http.Server{}
	srv.RegisterOnShutdown(func() {})
	srv.SetKeepAlivesEnabled(false)

	// text/template: AddParseTree and DefinedTemplates.
	tmpl := template.New("base")
	_, _ = tmpl.AddParseTree("sub", nil)
	_ = tmpl.DefinedTemplates()

	// reflect: TypeFor[T] (Go 1.22+).
	typ := reflect.TypeFor[int]()
	_ = typ
}
