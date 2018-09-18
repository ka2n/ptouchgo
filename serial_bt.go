package ptouchgo

import (
	"io"

	"github.com/goburrow/serial"
)

func OpenBluetooth(address string) (io.ReadWriteCloser, error) {
	return serial.Open(&serial.Config{
		Address:  address,
		BaudRate: 115200,
		StopBits: 1,
		Parity:   "N",
	})
}
