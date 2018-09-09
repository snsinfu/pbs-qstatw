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
	pbsBatchProtType       = 2
	pbsBatchProtVer        = 2
	pbsBatchStatusJob      = 19
	batchReplyChoiceStatus = 6
	bufferSize             = 1024
)

type Job struct {
	ID    string            `json:"id"`
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

	authAddr := ""
	if len(os.Args) == 2 {
		authAddr = os.Args[1]
	}

	serverAddr, err := torque.ActiveServer(authAddr)
	if err != nil {
		return err
	}

	conn, err := net.DialTCP("tcp", nil, serverAddr)
	if err != nil {
		return err
	}
	defer conn.Close()

	if err := torque.Authorize(conn, authAddr); err != nil {
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

		id, err := dis.ReadString(r)
		if err != nil {
			return nil, err
		}

		attrs, err := parseAttrList(r)
		if err != nil {
			return nil, err
		}

		jobs = append(jobs, Job{
			ID:    id,
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
