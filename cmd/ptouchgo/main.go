package main

import (
	"fmt"
	"os"

	"github.com/ka2n/ptouchgo"
)

func main() {
	if len(os.Args) <= 1 {
		panic("image file path required")
	}

	// prepare data
	imgFile, err := os.Open(os.Args[1])
	if err != nil {
		panic(err)
	}
	defer imgFile.Close()

	data, bytesWidth, err := ptouchgo.LoadRawImage(imgFile)
	if err != nil {
		panic(err)
	}

	// Compless data
	packedData, err := ptouchgo.CompressImage(data, bytesWidth)
	if err != nil {
		panic(err)
	}

	fmt.Println("Image loaded")

	// Open printer
	ser, err := ptouchgo.Open("/dev/rfcomm1", 24)
	if err != nil {
		panic(err)
	}
	defer ser.Close()

	ser.Initialize()
	fmt.Println(ser.DumpStatus())

	ser.Flush()
	ser.SetPTCBPMode()

	// Set property
	rasterLines := len(data) / bytesWidth
	ser.SetTapeProperty(rasterLines)

	// Transfer data
	ser.SendImage(packedData)

	// Dump status
	fmt.Println(ser.DumpStatus())

	// Re initialize
	ser.Initialize()
}
