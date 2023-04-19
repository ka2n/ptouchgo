package usb

import (
	"encoding/binary"
	"encoding/hex"
	"fmt"
	"io"
	"strings"
	"sync"

	"github.com/google/gousb"
	"github.com/ka2n/ptouchgo/conn"
)

const (
	brotherVendorID   = 0x04f9
	productIDPTP700   = 0x2061
	productIDPTP750W  = 0x2062
	productIDPTP710BT = 0x20af
)

type USBSerial struct {
	ctx    *gousb.Context
	dev    *gousb.Device
	mu     sync.Mutex
	readm  sync.Mutex
	writem sync.Mutex
	input  *gousb.InEndpoint
	output *gousb.OutEndpoint
	done   func()
}

func init() {
	conn.Register("usb", conn.DriverFunc(OpenUSB))
}

// OpenUSB open usb connection to device. if address is empty string, it will find pre defined device id.
// address should formatted like "20af" or empty string.
func OpenUSB(address string) (io.ReadWriteCloser, error) {
	var err error
	var ctx *gousb.Context
	var done func()
	var dev *gousb.Device
	var usbif *gousb.Interface
	var input *gousb.InEndpoint
	var output *gousb.OutEndpoint

	ctx = gousb.NewContext()
	ctx.Debug(10)

	if address != "" {
		if !strings.HasPrefix(address, "0x") {
			err = fmt.Errorf("invalid device address. address should \"0x0000\" form")
			goto handleError
		}

		var productID []byte
		productID, err = hex.DecodeString(address[2:])
		if err != nil {
			goto handleError
		}
		dev, err = ctx.OpenDeviceWithVIDPID(brotherVendorID, gousb.ID(binary.BigEndian.Uint16(productID)))
		if err != nil {
			goto handleError
		}
	} else {
		dev, _ = ctx.OpenDeviceWithVIDPID(brotherVendorID, productIDPTP750W)
		if dev == nil {
			dev, _ = ctx.OpenDeviceWithVIDPID(brotherVendorID, productIDPTP700)
		}

		if dev == nil {
			dev, _ = ctx.OpenDeviceWithVIDPID(brotherVendorID, productIDPTP710BT)
		}
	}

	if dev == nil {
		err = fmt.Errorf("USB device not found")
		goto handleError
	}

	err = dev.SetAutoDetach(true)
	if err != nil {
		err = fmt.Errorf("set auto detach kernel driver: %w", err)
		goto handleError
	}

	usbif, done, err = dev.DefaultInterface()
	if err != nil {
		err = fmt.Errorf("get default interface: %w", err)
		goto handleError
	}

	input, err = usbif.InEndpoint(0x81)
	if err != nil {
		err = fmt.Errorf("open InEndpoint: %w", err)
		goto handleError
	}

	output, err = usbif.OutEndpoint(0x02)
	if err != nil {
		err = fmt.Errorf("open OutEndpoint: %w", err)
		goto handleError
	}

	return &USBSerial{
		dev:    dev,
		input:  input,
		output: output,
		done: func() {
			done()
			dev.Close()
			ctx.Close()
		},
	}, nil

handleError:
	if done != nil {
		done()
	}
	if dev != nil {
		dev.Close()
	}
	if ctx != nil {
		ctx.Close()
	}
	return nil, err
}

func (s USBSerial) Close() error {
	s.mu.Lock()
	defer s.mu.Unlock()

	done := s.done
	s.done = nil
	s.input = nil
	s.output = nil
	done()
	return nil
}

func (s USBSerial) Write(b []byte) (int, error) {
	s.writem.Lock()
	defer s.writem.Unlock()
	return s.output.Write(b)
}

func (s USBSerial) Read(b []byte) (int, error) {
	s.readm.Lock()
	defer s.readm.Unlock()
	return s.input.Read(b)
}
