// net_rpc verifies that net/rpc is correctly intercepted.
//
// Expected: 0 violations.
package main

import (
	"net/rpc"
)

func main() {
	// net/rpc: Dial returns (*Client, error).
	client, err := rpc.Dial("tcp", "localhost:1234")
	_ = err

	if client == nil {
		// Intercept returns opaque non-nil — should not reach here.
		return
	}

	// net/rpc: *Client.Call returns error.
	var reply int
	err2 := client.Call("Arith.Multiply", nil, &reply)
	_ = err2

	// net/rpc: *Client.Go returns *rpc.Call.
	call := client.Go("Arith.Add", nil, &reply, nil)
	_ = call

	// net/rpc: *Client.Close returns error.
	_ = client.Close()

	// net/rpc: Register returns error.
	err3 := rpc.Register(nil)
	_ = err3

	// net/rpc: NewServer returns *Server.
	srv := rpc.NewServer()
	_ = srv
}
