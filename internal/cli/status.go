package cli

import (
	"context"
	"errors"
	"fmt"
	"io"
	"os"

	"github.com/alejandro/attack-shark-linux/internal/x6"
)

type StatusService interface {
	Status(context.Context) (x6.Status, error)
}

func Run(ctx context.Context, args []string, service StatusService, out io.Writer) int {
	if len(args) != 1 || args[0] != "status" {
		fmt.Fprint(out, "usage: attack-shark-linux status\n")
		return 2
	}
	status, err := service.Status(ctx)
	if err != nil {
		switch {
		case errors.Is(err, os.ErrPermission):
			fmt.Fprint(out, "permission denied\n")
			return 4
		case x6.IsErrorKind(err, x6.ReadFailure):
			fmt.Fprint(out, "input read failed\n")
			return 5
		default:
			fmt.Fprint(out, "no usable X6 device\n")
			return 3
		}
	}
	battery := "unavailable"
	if status.BatteryAvailable {
		battery = fmt.Sprintf("%d%%", status.BatteryPercent)
	}
	fmt.Fprintf(out, "status: connected\nconnection: %s\nbattery: %s\ndpi: unavailable\nrgb: unavailable\nfirmware: unavailable\n", status.Connection, battery)
	return 0
}
