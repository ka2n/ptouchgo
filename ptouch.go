// Package ptouchgo is a driver for PT-710BT/PT700/PT750W
package ptouchgo

import (
	"bytes"
	"encoding/hex"
	"fmt"
	"image"
	"image/png"
	"io"
	"log"

	"github.com/disintegration/imaging"
)

const (
	statusOffsetModel        = 4
	statusOffsetBattery      = 6
	statusOffsetErrorInfo1   = 8
	statusOffsetErrorInfo2   = 9
	statusOffsetMediaWidth   = 10
	statusOffsetMediaType    = 11
	statusOffsetMode         = 15
	statusOffsetTapeLength   = 17
	statusOffsetStatusType   = 18
	statusOffsetPhaseType    = 19
	statusOffsetPhaseNumber  = 20
	statusOffsetNotification = 22
	statusOffsetTapeColor    = 24
	statusOffsetFontColor    = 25
	statusOffsetHardwareConf = 26
)

type Status struct {
	Type         StatusType
	Model        Model
	Battery      BatteryStatusType
	Error1       Error1Type
	Error2       Error2Type
	Mode         int
	StatusType   StatusType
	PhaseType    PhaseTypeNumber
	Phase        PhaseNumber
	Notification Notification

	MediaType  MediaType
	TapeColor  TapeColor
	TapeLength int
	TapeWidth  TapeWidth
	FontColor  FontColor
}

//go:generate stringer -linecomment -type Model
type Model int

const (
        modelPTP700   Model = 0x67 // PT-P700
	modelPTP750W  Model = 0x68 // PT-P750W
	modelPTP710BT Model = 0x76 // PT-P710BT
)

type Error1Type int

const (
	error1NoMedia          Error1Type = 0x01 // No Media
	error1CutterJam        Error1Type = 0x04 // Cutter Jam
	error1WeakBattery      Error1Type = 0x08 // Weak battery
	error1TooHighVoltageAC Error1Type = 0x06 // Too high voltage from AC
)

//go:generate stringer -linecomment -type Error2Type
type Error2Type int

const (
	error2InvalidMedia Error2Type = 0x01 // Invalid media
	error2CoverOpen    Error2Type = 0x10 // Cover open
	error2Hot          Error2Type = 0x20 // Too hot
)

//go:generate stringer -linecomment -type TapeWidth
//go:generate go run ./internal/cmd/enum/enum.go -type TapeWidth
type TapeWidth int

const (
	tapeWidthNone TapeWidth = 0  // No tape
	tapeWidth3_5  TapeWidth = 4  // 3.5mm
	tapeWidth6    TapeWidth = 6  // 6mm
	tapeWidth9    TapeWidth = 9  // 9mm
	tapeWidth12   TapeWidth = 12 // 12mm
	tapeWidth18   TapeWidth = 18 // 18mm
	tapeWidth24   TapeWidth = 24 // 24mm
	tapeWidth62   TapeWidth = 62 // 62mm
)

//go:generate stringer -linecomment -type MediaType
type MediaType int

const (
	mediaTypeNone         MediaType = 0    // No tape
	mediaTypeLaminated    MediaType = 0x01 // Laminated
	mediaTypeNonLaminated MediaType = 0x03 // Non laminated
	mediaTypeHeatShirink  MediaType = 0x11 // Heat shrink tube
	mediaTypeInvalid      MediaType = 0xFF // Invalid tape type
)

//go:generate stringer -linecomment -type StatusType
type StatusType int

const (
	statusTypeReply             StatusType = 0    // Reply
	statusTypePrintingCompleted StatusType = 0x01 // Printing completed
	statusTypeErrorOccured      StatusType = 0x02 // Error occured
	statusTypeIFModeFinished    StatusType = 3    // IFModeFinished(unused)
	statusTypePowerOff          StatusType = 0x04 // Power off
	statusTypeNotification      StatusType = 0x05 // Notification
	statusTypePhaseChange       StatusType = 0x06 // Phase change
)

//go:generate stringer -trimprefix phaseType -type PhaseTypeNumber
type PhaseTypeNumber int

const (
	phaseTypeEdit   PhaseTypeNumber = 0x00 // Edit phase
	phaseTypeNormal PhaseTypeNumber = 0x01 // Normal phase
)

//go:generate stringer -trimprefix phaseNumber -type PhaseNumber
type PhaseNumber int

const (
	phaseNumberEdit     PhaseNumber = 0x00
	phaseNumberEditFeed PhaseNumber = 0x01

	phaseNumberNormal          PhaseNumber = 0x00
	phaseNumberNormalCoverOpen PhaseNumber = 0x14
)

//go:generate stringer -trimprefix notification -type Notification
type Notification int

const (
	notificationInvalid    Notification = 0x00
	notificationCoverOpen  Notification = 0x01
	notificationCoverClose Notification = 0x02
)

//go:generate stringer -trimprefix tapeColor -type TapeColor
type TapeColor int

const (
	tapeColorWhite             TapeColor = 0x01
	tapeColorOther             TapeColor = 0x02
	tapeColorClear             TapeColor = 0x03
	tapeColorRed               TapeColor = 0x04
	tapeColorBlue              TapeColor = 0x05
	tapeColorYellow            TapeColor = 0x06
	tapeColorGreen             TapeColor = 0x07
	tapeColorBlack             TapeColor = 0x08
	tapeColorClearWhiteText    TapeColor = 0x09
	tapeColorMatteWhite        TapeColor = 0x20
	tapeColorMatteClear        TapeColor = 0x21
	tapeColorMatteSilver       TapeColor = 0x22
	tapeColorSatinGold         TapeColor = 0x23
	tapeColorSatinSilver       TapeColor = 0x24
	tapeColorDBlue             TapeColor = 0x30 // TZe-535, TZe-545, TZe-555
	tapeColorDRed              TapeColor = 0x31 // TZe-435
	tapeColorFluorescentOrange TapeColor = 0x40
	tapeColorFluorescentyellow TapeColor = 0x41
	tapeColorBerryPink         TapeColor = 0x50 // TZe-MQP35
	tapeColorLightGray         TapeColor = 0x51 // TZe-MQL35
	tapeColorLimeGreen         TapeColor = 0x52 // TZe-MQG35
	tapeColorFYellow           TapeColor = 0x60
	tapeColorFPing             TapeColor = 0x61
	tapeColorFBlue             TapeColor = 0x62
	tapeColorHeatShrinkWhite   TapeColor = 0x70
	tapeColorFlexWhite         TapeColor = 0x90
	tapeColorFlexYellow        TapeColor = 0x91
	tapeColorCleaning          TapeColor = 0xF0
	tapeColorStencil           TapeColor = 0xF1
	tapeColorInvalid           TapeColor = 0xFF
)

//go:generate stringer -trimprefix fontColor -type FontColor
type FontColor int

const (
	fontColorWhite    FontColor = 0x01
	fontColorRed      FontColor = 0x04
	fontColorBlue     FontColor = 0x05
	fontColorBlack    FontColor = 0x08
	fontColorGold     FontColor = 0x0a
	fontColorFBlue    FontColor = 0x62
	fontColorCleaning FontColor = 0xF0
	fontColorStencil  FontColor = 0xF1
	fontColorOther    FontColor = 0x02
	fontColorInvalid  FontColor = 0xFF
)

//go:generate stringer -trimprefix battery -type BatteryStatusType
type BatteryStatusType int

const (
	batteryFull            BatteryStatusType = 0
	batteryHalf            BatteryStatusType = 1
	batteryLow             BatteryStatusType = 2
	batteryChangeBatteries BatteryStatusType = 3
	batteryAC              BatteryStatusType = 4
)

var (
	cmdInitialize               = []byte{0x1b, 0x40}
	cmdDumpStatus               = []byte{0x1b, 0x69, 0x53}
	cmdSetRasterMode            = []byte{0x1b, 0x69, 0x61, 0x01} // 0: ESC/P, 1: Raster, 3: P-touch Template, but only supported Raster
	cmdNotifyModePrefix         = []byte{0x1b, 0x69, 0x21}
	cmdSetPrintPropertyPrefix   = []byte{0x1b, 0x69, 0x7a}
	cmdSetPrintModePrefix       = []byte{0x1b, 0x69, 0x4d}
	cmdSetAutcutPrefix          = []byte{0x1b, 0x69, 0x41} // only for PT-P750W
	cmdSetExtendedModePrefix    = []byte{0x1b, 0x69, 0x4b}
	cmdSetFeedAmountPrefix      = []byte{0x1b, 0x69, 0x64}
	cmdSetCompressionModePrefix = []byte{0x4d}
	cmdRasterTransfer           = []byte{0x77}
	cmdRasterZeroline           = []byte{0x5a}
	cmdPrint                    = []byte{0x0c}
	cmdPrintAndEject            = []byte{0x1a}
)

const (
	printPropertyEnableBitMedia           = 0x02
	printPropertyEnableBitWidth           = 0x04
	printPropertyEnableBitLength          = 0x08
	printPropertyEnableBitQuality         = 0x40 // unused
	printPropertyEnableBitRecoverOnDevice = 0x80
)

type Serial struct {
	Conn        io.ReadWriteCloser
	TapeWidthMM uint
	Debug       bool
}

func Open(address string, TapeWidthMM uint, debug bool) (Serial, error) {
	var ser io.ReadWriteCloser
	var err error
	if address == "usb" {
		if debug {
			log.Println("Select USB driver")
		}
		ser, err = OpenUSB()
	} else {
		if debug {
			log.Println("Select Bluetooth driver")
		}
		ser, err = OpenBluetooth(address)
	}
	if err != nil {
		return Serial{}, err
	}
	return Serial{Conn: ser, TapeWidthMM: TapeWidthMM, Debug: debug}, err
}

// ClearBuffer clears current state
// If you want to stop ongoing data transfer,
// send ClearBuffer() and Initialize() then printer buffer are cleared and return to data receiving state
func (s Serial) ClearBuffer() error {
	// send empty instruction
	if s.Debug {
		log.Println("ClearBuffer")
	}
	_, err := s.Conn.Write(make([]byte, 100))
	return err
}

// Initialize clears mode setting
func (s Serial) Initialize() error {
	if s.Debug {
		log.Println("Initialize", hex.EncodeToString(cmdInitialize))
	}
	_, err := s.Conn.Write(cmdInitialize)
	return err
}

// RequestStatus requests current status
// do not use while printing
func (s Serial) RequestStatus() error {
	if s.Debug {
		log.Println("RequestStatus", hex.EncodeToString(cmdDumpStatus))
	}
	_, err := s.Conn.Write(cmdDumpStatus)
	return err
}

// ReadStatus reads current status from buffer
func (s Serial) ReadStatus() (*Status, error) {
	buf := make([]byte, 32)
	s.Conn.Read(buf)
	return parseStatus(buf)
}

func (s Serial) SetRasterMode() error {
	if s.Debug {
		log.Println("SetRasterMode", hex.EncodeToString(cmdSetRasterMode))
	}
	_, err := s.Conn.Write(cmdSetRasterMode)
	return err
}

// SetNotificationMode set auto status notification mode
// default: on
func (s Serial) SetNotificationMode(on bool) error {
	var b byte
	if on {
		b = 0x0
	} else {
		b = 0x1
	}

	payload := append(cmdNotifyModePrefix, b)
	if s.Debug {
		log.Println("SetNotificationMode", on, hex.EncodeToString(payload))
	}

	_, err := s.Conn.Write(payload)
	return err
}

func (s Serial) Close() error {
	return s.Conn.Close()
}

func (s Serial) SetPrintProperty(rasterLines int) error {
	var enableFlag int

	// 成功時: 1b697a860a3e00d00200000000
	// ON: 0x02, 0x04, 0x80
	// enableFlag |= 0x02
	// enableFlag |= 0x04
	// enableFlag |= 0x08
	// enableFlag |= 0x40
	// enableFlag |= 0x80

	enableFlag |= printPropertyEnableBitRecoverOnDevice

	// Tape
	tapeWidth := byte(uint(62))
	enableFlag |= printPropertyEnableBitWidth

	tapeLength := byte(uint(0))
	enableFlag |= printPropertyEnableBitLength

	// Data size
	// N4*256*256*256 + N3*256*256 + N2*256 + N1
	r := rasterLines
	rasterNumN4 := byte(r / (256 * 256 * 256))
	rasterNumN3 := byte(r % (256 * 256 * 256) / (256 * 256))
	rasterNumN2 := byte(r % (256 * 256 * 256) % (256 * 256) / 256)
	rasterNumN1 := byte(r % 256)

	// Media type
	const mediaType = byte(0x0A)
	enableFlag |= printPropertyEnableBitMedia

	const page = byte(0x00) // firstPage: 0, otherPage: 1

	const eeprom = byte(0x00)

	data := append(cmdSetPrintPropertyPrefix, []byte{
		byte(enableFlag),
		mediaType,
		tapeWidth,
		tapeLength,
		rasterNumN1,
		rasterNumN2,
		rasterNumN3,
		rasterNumN4,
		page,
		eeprom,
	}...)

	if s.Debug {
		log.Println("SetPrintProperty", hex.EncodeToString(data))
	}

	_, err := s.Conn.Write(data)
	return err
}

func (s Serial) SetPrintMode(autocut, mirror bool) error {
	var v int
	if autocut {
		v = setBit(v, 6)
	}
	// if mirror {
	// 	v = setBit(v, 7)
	// }
	payload := append(cmdSetPrintModePrefix, byte(v))
	if s.Debug {
		log.Println("SetPrintMode", hex.EncodeToString(payload))
	}

	_, err := s.Conn.Write(payload)
	return err
}

func (s Serial) SetExtendedMode(twoColorPrinting, cutAtEnd, highDefinitionPrinting bool) error {
	var v int

	// 2色ロールならtrue, 単色ロールならfalseで
	if twoColorPrinting {
		v = setBit(v, 0)
	}

	if cutAtEnd {
		v = setBit(v, 3)
	}

	if highDefinitionPrinting {
		v = setBit(v, 6)
	}

	payload := append(cmdSetExtendedModePrefix, byte(v))
	if s.Debug {
		log.Println("SetExtendedMode", hex.EncodeToString(payload))
	}

	_, err := s.Conn.Write(payload)
	return err
}

func (s Serial) SetFeedAmount(amount int) error {
	n1 := byte(amount % 256)
	n2 := byte(amount / 256)

	payload := append(cmdSetFeedAmountPrefix, []byte{
		n1, n2,
	}...)
	if s.Debug {
		log.Println("SetFeedAmount", hex.EncodeToString(payload))
	}
	_, err := s.Conn.Write(payload)
	return err
}

func (s Serial) SetAutocutPerPagesForPTP750W(pages int) error {
	if pages == 0 {
		pages = 1
	}
	payload := append(cmdSetAutcutPrefix, byte(pages))
	if s.Debug {
		log.Println("SetAutocutPerPagesForPTP750W", hex.EncodeToString(payload))
	}
	_, err := s.Conn.Write(payload)
	return err
}

func (s Serial) SetCompressionModeEnabled(enabled bool) error {
	var v byte
	if enabled {
		v = 0x02
	}

	payload := append(cmdSetCompressionModePrefix, v)
	if s.Debug {
		log.Println("SetCompressionModeEnabled", hex.EncodeToString(payload))
	}
	_, err := s.Conn.Write(payload)
	return err
}

func (s Serial) SendImage(tiffdata []byte) error {
	if s.Debug {
		log.Println("SendImage", len(tiffdata))
	}
	_, err := s.Conn.Write(tiffdata)
	return err
}

func (s Serial) Print() error {
	if s.Debug {
		log.Printf("Print %08b", cmdPrint)
	}
	_, err := s.Conn.Write(cmdPrint)
	return err
}

func (s Serial) PrintAndEject() error {
	if s.Debug {
		log.Printf("PrintAndEject %08b", cmdPrintAndEject)
	}
	_, err := s.Conn.Write(cmdPrintAndEject)
	return err
}

func (s Serial) Reset() error {
	err := s.ClearBuffer()
	if err != nil {
		return err
	}
	return s.Initialize()
}

func LoadPNGImage(r io.Reader, tapeWidth TapeWidth) ([]byte, int, error) {
	p, err := png.Decode(r)
	if err != nil {
		return nil, 0, err
	}
	return LoadRawImage(p, tapeWidth)
}

func LoadRawImage(p image.Image, tapeWidth TapeWidth) ([]byte, int, error) {
	ws := 720
	var canvas image.Image

	size := p.Bounds().Size()
	if size.X == ws {
		canvas = imaging.FlipH(p)
	} else if size.Y == ws {
		canvas = imaging.Transpose(p)
	} else {
		return nil, 0, fmt.Errorf("image size must have %dpx width or height for %d tape, got: %dx%d", ws, tapeWidth, size.X, size.Y)
	}

	size = canvas.Bounds().Size()
	bytesWidth := size.X / 8
	if size.X%8 != 0 {
		bytesWidth++
	}

	data := make([]byte, bytesWidth*size.Y)

	// 1bit
	for y := 0; y < size.Y; y++ {
		for x := 0; x < size.X; x++ {
			r, g, b, _ := canvas.At(x, y).RGBA()
			lightness := float64(55*r+182*g+18*b) / float64(0xffff*(55+182+18))
			if lightness <= 0.5 {
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

		fmt.Println(length)
		// fmt.Println(bytesWidth)

		dataBuf.Write(cmdRasterTransfer)
		dataBuf.Write([]byte{
			// byte(uint(length % 256)),
			// byte(uint(length / 256)),
			byte(0x02),
			byte(uint(bytesWidth)),
		})
		dataBuf.Write(chunk)
	}

	return dataBuf.Bytes(), nil
}

func parseStatus(in []byte) (*Status, error) {
	if len(in) != 32 {
		return nil, fmt.Errorf("status must be 32 bytes, got: %d", len(in))
	}

	return &Status{
		Type:         StatusType(in[statusOffsetStatusType]),
		Model:        Model(in[statusOffsetModel]),
		Battery:      BatteryStatusType(in[statusOffsetBattery]),
		Error1:       Error1Type(in[statusOffsetErrorInfo1]),
		Error2:       Error2Type(in[statusOffsetErrorInfo2]),
		Mode:         int(in[statusOffsetMode]),
		StatusType:   StatusType(in[statusOffsetStatusType]),
		PhaseType:    PhaseTypeNumber(in[statusOffsetPhaseType]),
		Phase:        PhaseNumber(in[statusOffsetPhaseNumber]),
		Notification: Notification(in[statusOffsetNotification]),
		MediaType:    MediaType(in[statusOffsetMediaType]),
		TapeColor:    TapeColor(in[statusOffsetTapeColor]),
		TapeLength:   int(in[statusOffsetTapeLength]),
		TapeWidth:    TapeWidth(in[statusOffsetMediaWidth]),
		FontColor:    FontColor(in[statusOffsetFontColor]),
	}, nil
}

func setBit(n int, pos uint) int {
	n |= (1 << pos)
	return n
}

func clearBit(n int, pos uint) int {
	mask := ^(1 << pos)
	n &= mask
	return n
}
