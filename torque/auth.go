package torque

import (
	"fmt"
	"net"
	"os"
	"os/user"
)

const (
	defaultAuthAddress = "/tmp/trqauthd-unix"
	trqAuthConnection  = 1
	trqGetActiveServer = 2
	authTypeIFF        = 1
	bufferSize         = 1024
)

// ActiveServer queries trqauthd for the address of the active PBS server.
func ActiveServer(authAddr string) (*net.TCPAddr, error) {
	if authAddr == "" {
		authAddr = defaultAuthAddress
	}

	auth, err := net.Dial("unix", authAddr)
	if err != nil {
		return nil, err
	}
	defer auth.Close()

	// Request
	enc := NewEncoder()
	enc.PutInt(trqGetActiveServer)
	msg := []byte(enc.String())

	if _, err := auth.Write(msg); err != nil {
		return nil, err
	}

	buf := make([]byte, bufferSize)
	n, err := auth.Read(buf)
	if err != nil {
		return nil, err
	}

	// Response: "err|host|port|"
	dec := NewDecoder(string(buf[:n]))

	respCode, err := dec.GetInt()
	if err != nil {
		return nil, err
	}

	if respCode != 0 {
		return nil, fmt.Errorf("code %d", respCode)
	}

	host, err := dec.GetString()
	if err != nil {
		return nil, err
	}

	port, err := dec.GetInt()
	if err != nil {
		return nil, err
	}

	return net.ResolveTCPAddr("tcp", fmt.Sprintf("%s:%d", host, port))
}

// Authorize grants authorization for given TCP connection to PBS server.
func Authorize(conn *net.TCPConn, authAddr string) error {
	if authAddr == "" {
		authAddr = defaultAuthAddress
	}

	auth, err := net.Dial("unix", authAddr)
	if err != nil {
		return err
	}
	defer auth.Close()

	me, err := user.Current()
	if err != nil {
		return err
	}
	username := me.Username
	pid := os.Getpid()
	port := conn.LocalAddr().(*net.TCPAddr).Port
	server := conn.RemoteAddr().(*net.TCPAddr)

	// Request
	enc := NewEncoder()
	enc.PutInt(trqAuthConnection)
	enc.PutString(server.IP.String())
	enc.PutInt(server.Port)
	enc.PutInt(authTypeIFF)
	enc.PutString(username)
	enc.PutInt(pid)
	enc.PutInt(port)
	msg := []byte(enc.String())

	if _, err := auth.Write(msg); err != nil {
		return err
	}

	buf := make([]byte, bufferSize)
	n, err := auth.Read(buf)
	if err != nil {
		return err
	}

	// Response
	dec := NewDecoder(string(buf[:n]))

	respCode, err := dec.GetInt()
	if err != nil {
		return err
	}

	if respCode != 0 {
		return fmt.Errorf("code %d", respCode)
	}

	return nil
}
