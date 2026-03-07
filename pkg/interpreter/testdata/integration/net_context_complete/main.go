// net_context_complete exercises additions from v0.84.0:
// net: (*Resolver).LookupIPAddr, (*TCPConn).SetKeepAlive/SetKeepAlivePeriod/SetNoDelay,
//      (*UDPConn).ReadFrom/WriteTo and typed variants;
// context: WithDeadlineCause, WithTimeoutCause (Go 1.21+), AfterFunc (Go 1.21+).
// Expected: 0 violations.
package main

import (
	"context"
	"net"
	"time"
)

func main() {
	// context.WithDeadlineCause and WithTimeoutCause (Go 1.21+).
	ctx, cancel := context.WithDeadlineCause(context.Background(), time.Now().Add(time.Second), nil)
	cancel()
	_ = ctx

	ctx2, cancel2 := context.WithTimeoutCause(context.Background(), time.Second, nil)
	cancel2()
	_ = ctx2

	// context.AfterFunc (Go 1.21+) — registers f to run when ctx is done.
	stop := context.AfterFunc(context.Background(), func() {})
	_ = stop

	// (*net.Resolver).LookupIPAddr.
	var r net.Resolver
	addrs, _ := r.LookupIPAddr(context.Background(), "localhost")
	_ = addrs

	// (*net.TCPConn) option methods.
	tc, _ := net.DialTCP("tcp", nil, nil)
	_ = tc.SetKeepAlive(true)
	_ = tc.SetKeepAlivePeriod(time.Second)
	_ = tc.SetNoDelay(true)

	// (*net.UDPConn) I/O methods.
	uc, _ := net.ListenUDP("udp", nil)
	buf := make([]byte, 64)
	n, addr, err := uc.ReadFrom(buf)
	_, _, _ = n, addr, err
	_, _ = uc.WriteTo(buf, nil)
}
