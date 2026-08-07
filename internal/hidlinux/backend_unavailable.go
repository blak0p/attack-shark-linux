//go:build !hidapi

package hidlinux

import (
	"errors"

	"github.com/alejandro/attack-shark-linux/internal/x6"
)

func NewPassiveTransport() (x6.PassiveInputTransport, error) {
	return nil, errors.New("HID backend requires the hidapi build tag")
}
