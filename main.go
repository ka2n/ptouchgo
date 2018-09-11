package main

import (
	"bytes"
	"fmt"
	"image/png"
	"log"
	"os"

	"github.com/goburrow/serial"
)

const (
	statusOffsetBattery       = 6
	statusOffsetExtendedError = 7
	statusOffsetErrorInfo1    = 8
	statusOffsetErrorInfo2    = 9
	statusOffsetStatusType    = 18
	statusOffsetPhaseType     = 19
	statusOffsetNotification  = 22
)

type Status struct {
	Type          StatusType
	Battery       BatteryStatusType
	ErrorCode1    int
	ErrorCode2    int
	ExtendedError int
}

type StatusType int

const (
	statusTypeReply             StatusType = 0
	statusTypePrintingCompleted StatusType = 1
	statusTypeErrorOccured      StatusType = 2
	statusTypeIFModeFinished    StatusType = 3
	statusTypePowerOff          StatusType = 4
	statusTypeNotification      StatusType = 5
	statusTypePhaseChange       StatusType = 6
)

func (s StatusType) String() string {
	switch s {
	case statusTypeReply:
		return "Reply to status request"
	case statusTypePrintingCompleted:
		return "Printing completed"
	case statusTypeErrorOccured:
		return "Error occured"
	case statusTypeIFModeFinished:
		return "IF mode finished"
	case statusTypePowerOff:
		return "Power off"
	case statusTypeNotification:
		return "Notification"
	case statusTypePhaseChange:
		return "Phase change"
	}
	return fmt.Sprintf("Status: %#x", s)
}

type BatteryStatusType int

const (
	batteryFull            BatteryStatusType = 0
	batteryHalf            BatteryStatusType = 1
	batteryLow             BatteryStatusType = 2
	batteryChangeBatteries BatteryStatusType = 3
	batteryAC              BatteryStatusType = 4
)

var (
	cmdInitialize   = []byte{0x1b, 0x40}
	cmdDumpStatus   = []byte{0x1b, 0x69, 0x53}
	cmdSetPTCBPMode = []byte{0x1b, 0x69, 0x61, 0x01}
	cmdTransfer     = []byte{0x47}
)

func (s BatteryStatusType) String() string {
	switch s {
	case batteryFull:
		return "Full"
	case batteryHalf:
		return "Half"
	case batteryLow:
		return "Low"
	case batteryChangeBatteries:
		return "Change batteries"
	case batteryAC:
		return "AC adapter in use"
	}
	return fmt.Sprintf("Battery: %#x", s)
}

func main() {
	// prepare data

	imgFile, err := os.Open("./out.png")
	if err != nil {
		panic(err)
	}
	defer imgFile.Close()

	originalPNG, err := png.Decode(imgFile)
	if err != nil {
		panic(err)
	}
	size := originalPNG.Bounds().Size()

	bytesWidth := size.X / 8
	if size.X%8 != 0 {
		bytesWidth++
	}
	data := make([]byte, bytesWidth*size.Y)

	for y := 0; y < size.Y; y++ {
		for x := 0; x < size.X; x++ {
			r, g, b, _ := originalPNG.At(x, y).RGBA()
			lightness := float64(55*r+182*g+18*b) / float64(0xffff*(55+182+18))
			if lightness > 0.5 {
				data[y*bytesWidth+x/8] |= 0x80 >> uint(x%8)
			}
		}
	}

	rasterLines := len(data) / bytesWidth
	chunkSize := bytesWidth

	// Compless data
	dataBuf := bytes.NewBuffer(nil)
	var n1 uint
	var n2 uint
	maxLen := len(data)
	for i := 0; i < maxLen; i += chunkSize {
		to := i + chunkSize
		if to > maxLen {
			to = maxLen
		}
		chunk := data[i:to]
		fmt.Println(i, chunkSize, to)

		packed, err := packBits(chunk)
		if err != nil {
			panic(err)
		}

		length := len(packed)
		fmt.Println(chunk)
		fmt.Println(packed)
		fmt.Println(length)

		n1 = uint(length % 256)
		n2 = uint(length / 256)

		dataBuf.Write(cmdTransfer)
		dataBuf.Write([]byte{byte(n1), byte(n2)})
		dataBuf.Write(packed)
	}

	// os.Exit(0)

	// Open printer
	ser, err := serial.Open(&serial.Config{
		Address:  "/dev/rfcomm0",
		BaudRate: 115200,
		StopBits: 1,
		Parity:   "N",
	})

	if err != nil {
		panic(err)
	}
	defer ser.Close()

	ser.Write(cmdInitialize)
	ser.Write(cmdDumpStatus)

	// Flush print buffer
	for i := 0; i < 64; i++ {
		ser.Write([]byte{0x00})
	}

	// Raster graphics(PTCBP) mode
	ser.Write(cmdSetPTCBPMode)

	buf := make([]byte, 32)
	ser.Read(buf)
	st, err := parseStatus(buf)
	if err != nil {
		panic(err)
	}
	if st != nil {
		log.Println(st)
	}

	tapeWidthInMM := 24

	// Set property
	fmt.Println("Set property")
	ser.Write([]byte{
		// Start set media quarity
		0x1b, 0x69, 0x7a,
		// #1: Print quality 0=fast, 1=high
		0xc4,
		// #2: Media type: 0=roll, 1=pre-cut labels
		0x01,
		// #3: Tape width in mm
		byte(tapeWidthInMM),
		// #4: Label height in mm (0 for roll)
		0x00,
		// #5, #6 Page consists of N = #5 + 256 * #6 pixel lines
		byte(uint(rasterLines % 256)),
		byte(uint(rasterLines / 256)),
		// Unused data bytes
		0x00, 0x00, 0x00, 0x00,
	})

	ser.Write([]byte{
		// # Set print chaining off (0x8) or on (0x0)
		0x1b, 0x69, 0x4b,
		0x08,
	})

	ser.Write([]byte{
		// # Set no mirror, no auto tape cut
		0x1b, 0x69, 0x4d,
		0x00,
	})

	ser.Write([]byte{
		// # Set margin amount (feed amount)
		0x1b, 0x69, 0x64,
		0x00, 0x00,
	})

	ser.Write([]byte{
		// Set compression mode: TIFF
		0x4d, 0x02,
	})

	// Transfer data
	// 128px @ 1bpp = 16 bytes

	ser.Write(dataBuf.Bytes())

	// Print and feed
	fmt.Println("Print")
	ser.Write([]byte{
		0x1a,
	})

	// Dump status
	fmt.Println("Dump status")
	buf = make([]byte, 32)
	ser.Read(buf)
	st, err = parseStatus(buf)
	if err != nil {
		panic(err)
	}
	if st != nil {
		log.Println(st)
	}

	// Re initialize
	ser.Write(cmdInitialize)
}

func parseStatus(in []byte) (*Status, error) {
	if len(in) != 32 {
		return nil, fmt.Errorf("status must be 32 bytes, got: %d", len(in))
	}

	return &Status{
		Type:          StatusType(in[statusOffsetStatusType]),
		Battery:       BatteryStatusType(in[statusOffsetBattery]),
		ErrorCode1:    int(in[statusOffsetErrorInfo1]),
		ErrorCode2:    int(in[statusOffsetErrorInfo2]),
		ExtendedError: int(in[statusOffsetExtendedError]),
	}, nil
}


func packBits(input []byte) ([]byte, error) {
	buf := bytes.NewBuffer(make([]byte, 0, 128))
	dst := make([]byte, 0, 1024)

	var rle bool
	var repeats int
	const maxRepeats = 127

	var finishRaw = func() {
		if buf.Len() == 0 {
			return
		}
		dst = append(dst, byte(buf.Len()-1))
		dst = append(dst, buf.Bytes()...)
		buf.Reset()
	}

	var finishRle = func(b byte, repeats int) {
		dst = append(dst, byte(256-(repeats-1)))
		dst = append(dst, b)
	}

	for i, b := range input {
		isLast := i == len(input)-1
		if isLast {
			if !rle {
				buf.WriteByte(b)
				finishRaw()
			} else {
				repeats++
				finishRle(b, repeats)
			}
			break
		}

		if b == input[i+1] {
			if !rle {
				finishRaw()
				rle = true
				repeats = 1
			} else {
				if repeats == maxRepeats {
					finishRle(b, repeats)
					repeats = 0
				}
				repeats++
			}
		} else {
			if !rle {
				if buf.Len() == maxRepeats {
					finishRaw()
				}
				buf.WriteByte(b)
			} else {
				repeats++
				finishRle(b, repeats)
				rle = false
				repeats = 0
			}
		}
	}
	return dst, nil
}
