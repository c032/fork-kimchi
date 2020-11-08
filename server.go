package main

import (
	"fmt"
	"log"
	"net"
	"net/http"
	"sync"

	"github.com/pires/go-proxyproto"
	"golang.org/x/net/http2"
)

type listenerKey struct {
	network string
	address string
}

type Server struct {
	listeners map[listenerKey]*Listener
}

func NewServer() *Server {
	return &Server{
		listeners: make(map[listenerKey]*Listener),
	}
}

func (srv *Server) Start() error {
	for _, ln := range srv.listeners {
		if err := ln.Start(); err != nil {
			return err
		}
	}

	return nil
}

func (srv *Server) AddListener(network, addr string) *Listener {
	k := listenerKey{network, addr}
	if ln, ok := srv.listeners[k]; ok {
		return ln
	}

	ln := newListener(srv, network, addr)
	srv.listeners[k] = ln
	return ln
}

type Listener struct {
	Network string
	Address string
	Mux     *http.ServeMux
	Server  *Server

	h1Server   *http.Server
	h1Listener *pipeListener

	h2Server *http2.Server
}

func newListener(srv *Server, network, addr string) *Listener {
	ln := &Listener{
		Network: network,
		Address: addr,
		Server:  srv,
	}
	ln.h1Listener = newPipeListener()
	ln.h1Server = &http.Server{Handler: ln}
	ln.h2Server = &http2.Server{}
	ln.Mux = http.NewServeMux()
	return ln
}

func (ln *Listener) Start() error {
	netLn, err := net.Listen(ln.Network, ln.Address)
	if err != nil {
		return err
	}
	log.Printf("HTTP server listening on %q", ln.Address)

	go func() {
		if err := ln.serve(netLn); err != nil {
			log.Fatalf("failed to serve listener %q: %v", ln.Address, err)
		}
	}()

	go func() {
		if err := ln.h1Server.Serve(ln.h1Listener); err != nil {
			log.Fatalf("HTTP/1 server: %v", err)
		}
	}()

	return nil
}

func (ln *Listener) serve(netLn net.Listener) error {
	for {
		conn, err := netLn.Accept()
		if err != nil {
			return fmt.Errorf("failed to accept connection: %v", err)
		}

		go func() {
			if err := ln.serveConn(conn); err != nil {
				log.Printf("listener %q: %v", ln.Address, err)
			}
		}()
	}
}

func (ln *Listener) serveConn(conn net.Conn) error {
	// TODO: only accept PROXY protocol from trusted sources
	var proto string
	proxyConn := proxyproto.NewConn(conn)
	if proxyHeader := proxyConn.ProxyHeader(); proxyHeader != nil {
		tlvs, err := proxyHeader.TLVs()
		if err != nil {
			conn.Close()
			return err
		}
		for _, tlv := range tlvs {
			if tlv.Type == proxyproto.PP2_TYPE_ALPN {
				proto = string(tlv.Value)
			}
		}
	}
	conn = proxyConn

	switch proto {
	case "h2", "h2c":
		defer conn.Close()
		opts := http2.ServeConnOpts{Handler: ln}
		ln.h2Server.ServeConn(conn, &opts)
		return nil
	case "", "http/1.0", "http/1.1":
		return ln.h1Listener.ServeConn(conn)
	default:
		conn.Close()
		return fmt.Errorf("unsupported protocol %q", proto)
	}
}

func (ln *Listener) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	ln.Mux.ServeHTTP(w, r)
}

var errPipeListenerClosed = fmt.Errorf("pipe listener closed")

// pipeListener is a hack to workaround the lack of http.Server.ServeConn.
// See: https://github.com/golang/go/issues/36673
type pipeListener struct {
	ch     chan net.Conn
	closed bool
	mu     sync.Mutex
}

func newPipeListener() *pipeListener {
	return &pipeListener{
		ch: make(chan net.Conn, 64),
	}
}

func (ln *pipeListener) Accept() (net.Conn, error) {
	conn, ok := <-ln.ch
	if !ok {
		return nil, errPipeListenerClosed
	}
	return conn, nil
}

func (ln *pipeListener) Close() error {
	ln.mu.Lock()
	defer ln.mu.Unlock()

	if ln.closed {
		return errPipeListenerClosed
	}
	ln.closed = true
	close(ln.ch)
	return nil
}

func (ln *pipeListener) ServeConn(conn net.Conn) error {
	ln.mu.Lock()
	defer ln.mu.Unlock()

	if ln.closed {
		return errPipeListenerClosed
	}
	ln.ch <- conn
	return nil
}

func (ln *pipeListener) Addr() net.Addr {
	return pipeAddr{}
}

type pipeAddr struct{}

func (pipeAddr) Network() string {
	return "pipe"
}

func (pipeAddr) String() string {
	return "pipe"
}
