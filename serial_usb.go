package ptouchgo

import (
	"fmt"
	"io"
	"sync"

	"github.com/google/gousb"
	"github.com/pkg/errors"
)

const (
	brotherVendorID   = 0x04f9
        productIDPTP700   = 0x2061
	productIDPTP750W  = 0x2062
	productIDPTP710BT = 0x20af
	productQL820NWB = 0x209d
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

func OpenUSB() (io.ReadWriteCloser, error) {
	var err error
	var ctx *gousb.Context
	var done func()
	var dev *gousb.Device
	var usbif *gousb.Interface
	var input *gousb.InEndpoint
	var output *gousb.OutEndpoint

	ctx = gousb.NewContext()
	ctx.Debug(10)

	dev, _ = ctx.OpenDeviceWithVIDPID(brotherVendorID, productIDPTP750W)
	if dev == nil {
		dev, _ = ctx.OpenDeviceWithVIDPID(brotherVendorID, productIDPTP700)
	}

        if dev == nil {
                dev, _ = ctx.OpenDeviceWithVIDPID(brotherVendorID, productIDPTP710BT)
				}
				
        if dev == nil {
                dev, _ = ctx.OpenDeviceWithVIDPID(brotherVendorID, productQL820NWB)
				}

	if dev == nil {
		err = fmt.Errorf("USB device not found")
		goto handleError
	}

	fmt.Println(dev)

	err = dev.SetAutoDetach(true)
	if err != nil {
		err = errors.Wrap(err, "set auto detach kernel driver")
		goto handleError
	}

	usbif, done, err = dev.DefaultInterface()
	if err != nil {
		err = errors.Wrap(err, "get default interface")
		goto handleError
	}

	input, err = usbif.InEndpoint(0x81)
	if err != nil {
		err = errors.Wrap(err, "open InEndpoint")
		goto handleError
	}

	output, err = usbif.OutEndpoint(0x02)
	if err != nil {
		err = errors.Wrap(err, "open OutEndpoint")
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
