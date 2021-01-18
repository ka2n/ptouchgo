package conn

import (
	"io"
	"net"

	"github.com/goburrow/serial"
)

// openSerial for generic serial connection
func openSerial(address string) (io.ReadWriteCloser, error) {
	return serial.Open(&serial.Config{
		Address:  address,
		BaudRate: 115200,
		StopBits: 1,
		Parity:   "N",
	})
}

func openTCP(address string) (io.ReadWriteCloser, error) {
	return net.Dial("tcp", address)
}
