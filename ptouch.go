package ptouchgo

import (
	"bytes"
	"fmt"
	"image/png"
	"io"

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

type Serial struct {
	Conn        io.ReadWriteCloser
	TapeWidthMM uint
}

func Open(address string, TapeWidthMM uint) (Serial, error) {
	ser, err := serial.Open(&serial.Config{
		Address:  address,
		BaudRate: 115200,
		StopBits: 1,
		Parity:   "N",
	})
	return Serial{Conn: ser, TapeWidthMM: TapeWidthMM}, err
}

func (s Serial) Initialize() error {
	_, err := s.Conn.Write(cmdInitialize)
	return err
}

func (s Serial) Flush() {
	// Flush print buffer
	for i := 0; i < 64; i++ {
		s.Conn.Write([]byte{0x00})
	}
}

func (s Serial) DumpStatus() (*Status, error) {
	_, err := s.Conn.Write(cmdDumpStatus)
	if err != nil {
		return nil, err
	}

	buf := make([]byte, 32)
	s.Conn.Read(buf)
	return parseStatus(buf)
}

func (s Serial) SetPTCBPMode() {
	s.Conn.Write(cmdSetPTCBPMode)
}

func (s Serial) Close() error {
	return s.Conn.Close()
}

func (s Serial) SetTapeProperty(rasterLines int) error {
	_, err := s.Conn.Write([]byte{
		// Start set media quarity
		0x1b, 0x69, 0x7a,
		// #1: Print quality 0=fast, 1=high
		0xc4,
		// #2: Media type: 0=roll, 1=pre-cut labels
		0x01,
		// #3: Tape width in mm
		byte(s.TapeWidthMM),
		// #4: Label height in mm (0 for roll)
		0x00,
		// #5, #6 Page consists of N = #5 + 256 * #6 pixel lines
		byte(uint(rasterLines % 256)),
		byte(uint(rasterLines / 256)),
		// Unused data bytes
		0x00, 0x00, 0x00, 0x00,
	})
	if err != nil {
		return err
	}

	_, err = s.Conn.Write([]byte{
		// # Set no mirror, no auto tape cut
		0x1b, 0x69, 0x4d,
		0x00,
	})
	if err != nil {
		return err
	}

	_, err = s.Conn.Write([]byte{
		// # Set print chaining off (0x8) or on (0x0)
		0x1b, 0x69, 0x4b,
		0x08,
	})
	if err != nil {
		return err
	}

	_, err = s.Conn.Write([]byte{
		// # Set margin amount (feed amount)
		0x1b, 0x69, 0x64,
		0x00, 0x00,
	})
	if err != nil {
		return err
	}

	_, err = s.Conn.Write([]byte{
		// Set compression mode: TIFF
		0x4d, 0x02,
	})
	if err != nil {
		return err
	}

	return nil
}

func (s Serial) SendImage(tiffdata []byte) error {
	// 128px @ 1bpp = 16 bytes
	_, err := s.Conn.Write(tiffdata)
	if err != nil {
		return err
	}

	// Print and feed
	_, err = s.Conn.Write([]byte{
		0x1a,
	})
	return err
}

func (s Serial) Reset() error {
	return s.Initialize()
}

func LoadRawImage(r io.Reader) ([]byte, int, error) {
	originalPNG, err := png.Decode(r)
	if err != nil {
		return nil, 0, err
	}
	size := originalPNG.Bounds().Size()
	if size.X != 128 {
		return nil, 0, fmt.Errorf("image size must have 128px width, got: %d", size.X)
	}

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

	return data, bytesWidth, nil
}

func CompressImage(data []byte, bytesWidth int) ([]byte, error) {
	var dataBuf bytes.Buffer
	max := len(data)

	for i := 0; i < max; i += bytesWidth {
		to := i + bytesWidth
		if to > max {
			to = max
		}
		chunk := data[i:to]

		packed, err := packBits(chunk)
		if err != nil {
			return nil, err
		}

		length := len(packed)

		dataBuf.Write(cmdTransfer)
		dataBuf.Write([]byte{
			byte(uint(length % 256)),
			byte(uint(length / 256)),
		})
		dataBuf.Write(packed)
	}

	return dataBuf.Bytes(), nil
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
