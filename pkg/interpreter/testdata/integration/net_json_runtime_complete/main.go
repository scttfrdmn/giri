// net_json_runtime_complete exercises additions from v0.81.0:
// net: Listener.Accept/Addr, typed dial/listen (DialTCP/UDP/Unix,
//      ListenTCP/UDP/Unix), (*Dialer).DialContext;
// encoding/json: json.Number.Float64 and json.Number.Int64;
// runtime: (*Func).Name, (*Func).Entry, (*Func).FileLine.
// Expected: 0 violations.
package main

import (
	"context"
	"encoding/json"
	"net"
	"runtime"
)

func main() {
	// net.Listen + Listener.Accept, Addr, Close.
	l, _ := net.Listen("tcp", ":0")
	_ = l.Addr()
	conn, _ := l.Accept()
	_ = conn
	_ = l.Close()

	// Typed listen variants.
	lt, _ := net.ListenTCP("tcp", nil)
	_ = lt
	lu, _ := net.ListenUDP("udp", nil)
	_ = lu
	lx, _ := net.ListenUnix("unix", nil)
	_ = lx

	// Typed dial variants.
	ct, _ := net.DialTCP("tcp", nil, nil)
	_ = ct
	cu, _ := net.DialUDP("udp", nil, nil)
	_ = cu
	cx, _ := net.DialUnix("unix", nil, nil)
	_ = cx

	// (*net.Dialer).DialContext.
	var d net.Dialer
	dc, _ := d.DialContext(context.Background(), "tcp", "localhost:80")
	_ = dc

	// json.Number methods.
	var n json.Number = "42"
	f, _ := n.Float64()
	_ = f
	i, _ := n.Int64()
	_ = i

	// runtime.FuncForPC + (*Func) methods.
	pc, _, _, _ := runtime.Caller(0)
	fn := runtime.FuncForPC(uintptr(pc))
	_ = fn.Name()
	_ = fn.Entry()
	_, _ = fn.FileLine(uintptr(pc))
}
