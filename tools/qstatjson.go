package main

import (
	"bufio"
	"encoding/json"
	"fmt"
	"net"
	"os"
	"os/user"

	"github.com/snsinfu/pbs-qstatw/dis"
	"github.com/snsinfu/pbs-qstatw/torque"
)

const (
	defaultAuthAddress     = "/tmp/trqauthd-unix"
	trqAuthConnection      = 1
	trqGetActiveServer     = 2
	authTypeIFF            = 1
	pbsBatchProtType       = 2
	pbsBatchProtVer        = 2
	pbsBatchStatusJob      = 19
	batchReplyChoiceStatus = 6
	bufferSize             = 1024
)

type Job struct {
	Name  string            `json:"name"`
	Attrs map[string]string `json:"attributes"`
}

func main() {
	if err := run(); err != nil {
		fmt.Fprintln(os.Stderr, "error:", err)
		os.Exit(1)
	}
}

func run() error {
	if len(os.Args) > 2 {
		fmt.Fprintln(os.Stderr, "usage: qstatjson [socket]")
		os.Exit(64)
	}

	authAddr := defaultAuthAddress
	if len(os.Args) == 2 {
		authAddr = os.Args[1]
	}

	serverAddr, err := queryActiveServer(authAddr)
	if err != nil {
		return err
	}

	conn, err := net.DialTCP("tcp", nil, serverAddr)
	if err != nil {
		return err
	}
	defer conn.Close()

	if err := authorizeConnection(conn, authAddr); err != nil {
		return err
	}

	r := bufio.NewReader(conn)
	w := bufio.NewWriter(conn)

	jobs, err := queryJobStatus(r, w)
	if err != nil {
		return err
	}

	data, err := json.Marshal(jobs)
	if err != nil {
		return err
	}

	fmt.Print(string(data))

	return nil
}

func queryJobStatus(r *bufio.Reader, w *bufio.Writer) ([]Job, error) {
	me, err := user.Current()
	if err != nil {
		return nil, err
	}
	username := me.Username

	w.WriteString(dis.EncodeInt(pbsBatchProtType))
	w.WriteString(dis.EncodeInt(pbsBatchProtVer))
	w.WriteString(dis.EncodeInt(pbsBatchStatusJob))
	w.WriteString(dis.EncodeString(username))
	w.WriteString(dis.EncodeString(""))
	w.WriteString(dis.EncodeInt(0))
	w.WriteString(dis.EncodeInt(0))

	if err := w.Flush(); err != nil {
		return nil, err
	}

	// Parse response
	choice, err := parseResponseHeader(r)
	if err != nil {
		return nil, err
	}

	if choice != batchReplyChoiceStatus {
		return nil, fmt.Errorf("unrecognized choice=%d", choice)
	}

	jobCount, err := dis.ReadInt(r)
	if err != nil {
		return nil, err
	}

	jobs := []Job{}

	for i := 0; i < int(jobCount); i++ {
		if _, err := dis.ReadInt(r); err != nil {
			return nil, err
		}

		name, err := dis.ReadString(r)
		if err != nil {
			return nil, err
		}

		attrs, err := parseAttrList(r)
		if err != nil {
			return nil, err
		}

		jobs = append(jobs, Job{
			Name:  name,
			Attrs: attrs,
		})
	}

	return jobs, nil
}

func parseAttrList(r *bufio.Reader) (map[string]string, error) {
	count, err := dis.ReadInt(r)
	if err != nil {
		return nil, err
	}

	attrs := map[string]string{}

	for i := 0; i < int(count); i++ {
		if _, err := dis.ReadInt(r); err != nil {
			return nil, err
		}

		name, err := dis.ReadString(r)
		if err != nil {
			return nil, err
		}

		hasRes, err := dis.ReadInt(r)
		if err != nil {
			return nil, err
		}

		if hasRes != 0 {
			res, err := dis.ReadString(r)
			if err != nil {
				return nil, err
			}
			name += "." + res
		}

		value, err := dis.ReadString(r)
		if err != nil {
			return nil, err
		}

		if _, err := dis.ReadInt(r); err != nil {
			return nil, err
		}

		attrs[name] = value
	}

	return attrs, nil
}

func parseResponseHeader(r *bufio.Reader) (int, error) {
	resType, err := dis.ReadInt(r)
	if err != nil {
		return -1, err
	}

	resVer, err := dis.ReadInt(r)
	if err != nil {
		return -1, err
	}

	if resType != pbsBatchProtType || resVer != pbsBatchProtVer {
		return -1, fmt.Errorf("unrecognized protocol: type=%d ver=%d", resType, resVer)
	}

	resCode, err := dis.ReadInt(r)
	if err != nil {
		return -1, err
	}

	resAux, err := dis.ReadInt(r)
	if err != nil {
		return -1, err
	}

	if resCode != 0 {
		return -1, fmt.Errorf("code=%d aux=%d", resCode, resAux)
	}

	resChoice, err := dis.ReadInt(r)
	if err != nil {
		return -1, err
	}

	return int(resChoice), nil
}

func authorizeConnection(conn *net.TCPConn, authAddr string) error {
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

	server := conn.RemoteAddr().(*net.TCPAddr)
	port := conn.LocalAddr().(*net.TCPAddr).Port

	enc := torque.NewEncoder()
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

	dec := torque.NewDecoder(string(buf[:n]))

	respCode, err := dec.GetInt()
	if err != nil {
		return err
	}

	if respCode != 0 {
		return fmt.Errorf("code %d", respCode)
	}

	return nil
}

func queryActiveServer(authAddr string) (*net.TCPAddr, error) {
	auth, err := net.Dial("unix", authAddr)
	if err != nil {
		return nil, err
	}
	defer auth.Close()

	enc := torque.NewEncoder()
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

	// Receive response: "err|host|port|"
	dec := torque.NewDecoder(string(buf[:n]))

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
