package main

import (
	"fmt"
	"log"
	"time"

	"github.com/google/gousb"
)

// https://pkg.go.dev/github.com/google/gousb
// https://www.beyondlogic.org/usbnutshell/usb1.shtml
// may need to modprobe -r usbtmc if there are device busy errors

func main() {
	// Initialize a new Context.
	ctx := gousb.NewContext()
	defer ctx.Close()

	// This vid/pid is for Rigol Technologies DS1xx4Z/MSO1xxZ series
	// requires permissions to be opened up on /dev/usbtmc1 via udev?
	dev, err := ctx.OpenDeviceWithVIDPID(0x1ab1, 0x04ce)
	if dev == nil {
		log.Fatal("Device not found\n")
	}
	if err != nil {
		log.Fatalf("Could not open a device: %v", err)
	}
	defer dev.Close()

	// Claim the default interface using a convenience function.
	// The default interface is always #0 alt #0 in the currently active
	// config.
	intf, done, err := dev.DefaultInterface()
	if err != nil {
		log.Fatalf("%s.DefaultInterface(): %v", dev, err)
	}
	defer done()

	// Open an OUT endpoint.
	epOut, err := intf.OutEndpoint(3)
	if err != nil {
		log.Fatalf("%s.OutEndpoint(3): %v", intf, err)
	}

	// In this interface open endpoint #1 for reading.
	epIn, err := intf.InEndpoint(1)
	if err != nil {
		log.Fatalf("%s.InEndpoint(1): %v", intf, err)
	}

	// Write data to the USB device.
	numBytes, err := epOut.Write([]byte("*IDN?"))
	if numBytes != 5 {
		log.Fatalf("%s.Write([5]): only %d bytes written, returned error is %v", epOut, numBytes, err)
	}
	fmt.Println("5 bytes successfully sent to the endpoint")

	// Buffer large enough for 10 USB packets from endpoint 6.
	buf := make([]byte, 10*epIn.Desc.MaxPacketSize)
	time.Sleep(1 * time.Second)
	// readBytes might be smaller than the buffer size. readBytes might be greater than zero even if err is not nil.
	readBytes, err := epIn.Read(buf)
	if err != nil {
		fmt.Println("Read returned an error:", err)
	}
	fmt.Printf("Read %d bytes\n", readBytes)
	fmt.Println(string(buf))
}
