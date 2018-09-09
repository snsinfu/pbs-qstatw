package main

import (
	"fmt"
	"os"

	"github.com/docopt/docopt-go"
	"github.com/snsinfu/pbs-qstatw/qstatw"
)

const usage = `
Monitor PBS jobs.

Usage:
  qstatw [-h -a <socket>]

Options:
  --auth, -a <socket>  Set auth socket [default: /tmp/trqauthd-unix]
  --help, -h           Show this help message and exit
`

func main() {
	opts, err := docopt.ParseDoc(usage)
	if err != nil {
		panic(err)
	}

	config := qstatw.Config{
		AuthAddr: opts["--auth"].(string),
	}

	if err := qstatw.Run(config); err != nil {
		fmt.Fprintln(os.Stderr, "error:", err)
		os.Exit(1)
	}
}
