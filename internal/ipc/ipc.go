// Package ipc implements the small JSON-over-unix-socket protocol termlog uses so the zsh preexec/precmd hooks (and the `termlog offline/online/quit/status` commands) can talk to the running recorder for the current terminal tab.
package ipc

import (
	"bufio"
	"encoding/json"
	"fmt"
	"net"
	"os"
	"time"
)

type Request struct {
	Cmd  string `json:"cmd"`
	Arg  string `json:"arg,omitempty"`
	PWD  string `json:"pwd,omitempty"`
	Host string `json:"host,omitempty"`
	User string `json:"user,omitempty"`
}

type Response struct {
	OK    bool            `json:"ok"`
	Error string          `json:"error,omitempty"`
	Data  json.RawMessage `json:"data,omitempty"`
}

// Serve starts accepting connections on sockPath, calling handle for each decoded request and writing back the returned response.
func Serve(sockPath string, handle func(Request) Response) (net.Listener, error) {
	os.Remove(sockPath)
	l, err := net.Listen("unix", sockPath)
	if err != nil {
		return nil, err
	}
	go func() {
		for {
			conn, err := l.Accept()
			if err != nil {
				return // listener closed
			}
			go func(c net.Conn) {
				defer c.Close()
				dec := json.NewDecoder(c)
				var req Request
				if err := dec.Decode(&req); err != nil {
					return
				}
				resp := handle(req)
				enc := json.NewEncoder(c)
				enc.Encode(resp)
			}(conn)
		}
	}()
	return l, nil
}

// Send connects to sockPath, sends req, and returns the decoded response.
func Send(sockPath string, req Request) (Response, error) {
	conn, err := net.DialTimeout("unix", sockPath, 500*time.Millisecond)
	if err != nil {
		return Response{}, fmt.Errorf("no termlog session on this terminal: %w", err)
	}
	defer conn.Close()
	if err := json.NewEncoder(conn).Encode(req); err != nil {
		return Response{}, err
	}
	conn.SetReadDeadline(time.Now().Add(2 * time.Second))
	var resp Response
	if err := json.NewDecoder(bufio.NewReader(conn)).Decode(&resp); err != nil {
		return Response{}, err
	}
	return resp, nil
}
