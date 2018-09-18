package main

import (
	"flag"
	"fmt"
	"log"
	"os"

	"github.com/ka2n/ptouchgo"
	"github.com/pkg/errors"
)

var (
	imagePath  = flag.String("i", "", "Image path")
	devicePath = flag.String("d", "/dev/rfcomm0", "Device path(RFCOMM or \"usb\")")
	tapeWidth  = flag.Uint("t", 24, "Tape width")
	debugMode  = flag.Bool("debug", false, "Debug decoded image")
	dryRunMode = flag.Bool("dry", false, "not printing")
)

var (
	ser ptouchgo.Serial
)

func main() {
	log.SetPrefix("ptouchgo: ")
	log.SetFlags(0)
	flag.Parse()

	err := mainCLI()
	if err != nil {
		log.Fatalln(err)
	}
}

func mainCLI() error {

	var err error
	if *imagePath == "" || *devicePath == "" {
		return fmt.Errorf("image file path and device path required")
	}

	tw := ptouchgo.TapeWidth(*tapeWidth)
	if !tw.Valid() {
		return fmt.Errorf("tapeWith only accespts 3.5,6,9,12,18,24")
	}

	// prepare data
	imgFile, err := os.Open(*imagePath)
	if err != nil {
		return err
	}
	defer imgFile.Close()

	data, bytesWidth, err := ptouchgo.LoadPNGImage(imgFile, tw)
	if err != nil {
		return errors.Wrap(err, "load image")
	}
	rasterLines := len(data) / bytesWidth

	debug := *debugMode
	if debug {
		for i := 0; i < len(data); i += bytesWidth {
			to := i + bytesWidth
			if to > len(data) {
				to = len(data)
			}
			chunk := data[i:to]
			for _, c := range chunk {
				fmt.Printf("%08b", c)
			}
			fmt.Println()
		}
	}

	// Compless data
	packedData, err := ptouchgo.CompressImage(data, bytesWidth)
	if err != nil {
		return errors.Wrap(err, "convert image")
	}

	if debug {
		log.Println("Image loaded")
	}

	// Open printer
	ser, err = ptouchgo.Open(*devicePath, *tapeWidth, debug)
	if err != nil {
		return errors.Wrap(err, *devicePath)
	}
	defer ser.Close()

	err = ser.Reset()
	if err != nil {
		return err
	}

	err = ser.SetRasterMode()
	if err != nil {
		return err
	}

	// Set property
	err = ser.SetPrintProperty(rasterLines)
	if err != nil {
		return err
	}

	err = ser.SetPrintMode(true, false)
	if err != nil {
		return err
	}

	err = ser.SetExtendedMode(false, true, false, false, false)
	if err != nil {
		return err
	}

	err = ser.SetFeedAmount(10)
	if err != nil {
		return err
	}

	err = ser.SetCompressionModeEnabled(true)
	if err != nil {
		return err
	}

	if !*dryRunMode {
		err = ser.SendImage(packedData)
		if err != nil {
			return err
		}
	}

	if !*dryRunMode {
		err = ser.PrintAndEject()
		if err != nil {
			return err
		}
	}

	ser.Reset()
	return nil
}
