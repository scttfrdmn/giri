// http_net_os_unsafe_complete exercises additions from v0.87.0:
// net/http: (*Client).CloseIdleConnections, (*Transport).CloseIdleConnections,
//           (*Request).FormFile, (*Request).MultipartReader;
// net: (*Interface).Addrs, (*Interface).MulticastAddrs;
// os: (*File).SyscallConn;
// unsafe: SliceData, StringData (Go 1.20+ builtins).
// Expected: 0 violations.
package main

import (
	"net"
	"net/http"
	"os"
	"unsafe"
)

func main() {
	// net/http: (*Client).CloseIdleConnections.
	client := &http.Client{}
	client.CloseIdleConnections()

	// net/http: (*Transport).CloseIdleConnections.
	tr := &http.Transport{}
	tr.CloseIdleConnections()

	// net/http: (*Request).FormFile and (*Request).MultipartReader.
	req, _ := http.NewRequest("POST", "http://example.com/upload", nil)
	f, fh, err := req.FormFile("file")
	_, _, _ = f, fh, err
	mr, _ := req.MultipartReader()
	_ = mr

	// net: (*Interface).Addrs and MulticastAddrs.
	ifaces, _ := net.Interfaces()
	for _, iface := range ifaces {
		addrs, _ := iface.Addrs()
		_ = addrs
		maddrs, _ := iface.MulticastAddrs()
		_ = maddrs
	}

	// os: (*File).SyscallConn.
	f2, _ := os.Open(os.DevNull)
	if f2 != nil {
		rc, _ := f2.SyscallConn()
		_ = rc
		f2.Close()
	}

	// unsafe: SliceData and StringData (Go 1.20+ builtins).
	s := []byte{1, 2, 3}
	ptr := unsafe.SliceData(s)
	_ = ptr

	str := "hello"
	bptr := unsafe.StringData(str)
	_ = bptr
}
