package main

import (
	"context"
	"fmt"
	"io"
	"os"

	"github.com/alejandro/attack-shark-linux/internal/cli"
	"github.com/alejandro/attack-shark-linux/internal/hidlinux"
	"github.com/alejandro/attack-shark-linux/internal/x6"
)

func main() {
	transport, err := hidlinux.NewPassiveTransport()
	if err != nil {
		fmt.Fprint(os.Stdout, "no usable X6 device\n")
		os.Exit(3)
	}
	os.Exit(run(os.Args[1:], x6.NewService(transport), os.Stdout))
}

func run(args []string, service cli.StatusService, out io.Writer) int {
	return cli.Run(context.Background(), args, service, out)
}
