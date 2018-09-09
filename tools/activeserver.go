package main

import (
	"fmt"
	"net"
	"os"
	"time"

	"github.com/snsinfu/pbs-qstatw/torque"
)

const (
	defaultAuthAddress = "/tmp/trqauthd-unix"
	trqGetActiveServer = 2
	bufferSize         = 1024
	timeout            = 5 * time.Second
)

func main() {
	if err := run(); err != nil {
		fmt.Fprintln(os.Stderr, "error:", err)
		os.Exit(1)
	}
}

func run() error {
	if len(os.Args) > 2 {
		fmt.Fprintln(os.Stderr, "usage: activeserver [socket]")
		os.Exit(64)
	}

	// Connect to trqauthd
	authAddress := defaultAuthAddress
	if len(os.Args) == 2 {
		authAddress = os.Args[1]
	}

	conn, err := net.Dial("unix", authAddress)
	if err != nil {
		return err
	}

	if err := conn.SetDeadline(time.Now().Add(timeout)); err != nil {
		return err
	}

	// Send GET_ACTIVE_SERVER request: "2|"
	enc := torque.NewEncoder()
	enc.PutInt(trqGetActiveServer)
	msg := enc.String()

	if _, err := conn.Write([]byte(msg)); err != nil {
		return err
	}

	buf := make([]byte, bufferSize)
	n, err := conn.Read(buf)
	if err != nil {
		return err
	}

	// Receive response: "err|host|port|"
	dec := torque.NewDecoder(string(buf[:n]))

	respCode, err := dec.GetInt()
	if err != nil {
		return err
	}

	if respCode != 0 {
		return fmt.Errorf("code %d", respCode)
	}

	host, err := dec.GetString()
	if err != nil {
		return err
	}

	port, err := dec.GetInt()
	if err != nil {
		return err
	}

	fmt.Printf("%s:%d\n", host, port)

	return nil
}
