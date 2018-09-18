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
	rfcommPath = flag.String("d", "/dev/rfcomm0", "Device path(RFCOMM)")
	tapeWidth  = flag.Uint("t", 24, "Tape width")
	debugMode  = flag.Bool("debug", false, "Debug decoded image")
)

var (
	ser ptouchgo.Serial
)

func main() {
	var err error
	log.SetPrefix("ptouchgo")
	log.SetFlags(0)

	flag.Parse()
	if *imagePath == "" || *rfcommPath == "" {
		log.Fatalln("image file path and rcomm device path required")
	}

	tw := ptouchgo.TapeWidth(*tapeWidth)
	if !tw.Valid() {
		log.Fatalln("tapeWith only accespts 3.5,6,9,12,18,24")
	}

	// prepare data
	imgFile, err := os.Open(*imagePath)
	if err != nil {
		log.Fatalln(err)
	}
	defer imgFile.Close()

	data, bytesWidth, err := ptouchgo.LoadRawImage(imgFile, tw)
	if err != nil {
		log.Fatalln(errors.Wrap(err, "load image"))
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
		os.Exit(0)
	}

	// Compless data
	packedData, err := ptouchgo.CompressImage(data, bytesWidth)
	if err != nil {
		log.Fatalln(errors.Wrap(err, "convert image"))
	}

	log.Println("Image loaded")

	// Open printer
	ser, err = ptouchgo.Open(*rfcommPath, *tapeWidth)
	if err != nil {
		log.Fatalln(errors.Wrap(err, *rfcommPath))
	}
	defer ser.Close()

	err = ser.Reset()
	if err != nil {
		goto handleError
	}

	err = ser.SetRasterMode()
	if err != nil {
		goto handleError
	}

	// Set property
	err = ser.SetPrintProperty(rasterLines)
	if err != nil {
		goto handleError
	}

	err = ser.SetPrintMode(true, false)
	if err != nil {
		goto handleError
	}

	err = ser.SetExtendedMode(false, true, false, false, false)
	if err != nil {
		goto handleError
	}

	err = ser.SetFeedAmount(1)
	if err != nil {
		goto handleError
	}

	err = ser.SetCompressionModeEnabled(true)
	if err != nil {
		goto handleError
	}

	err = ser.SendImage(packedData)
	if err != nil {
		goto handleError
	}

	err = ser.PrintAndEject()
	if err != nil {
		goto handleError
	}

	ser.Reset()
	return

handleError:
	ser.Close()
	log.Fatalln(err)
}
