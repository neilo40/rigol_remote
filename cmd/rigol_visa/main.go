package main

import (
	"errors"
	"fmt"
	"log"
	"strconv"
	"strings"
	"time"

	vi "github.com/jpoirier/visa"
)

// Refer to https://www.batronix.com/files/Rigol/Oszilloskope/_DS&MSO1000Z/MSO_DS1000Z_ProgrammingGuide_EN.pdf

type Rigol struct {
	Instr           vi.Object
	ResourceManager vi.Session
}

func (r *Rigol) Init(connStr string) error {
	rm, status := vi.OpenDefaultRM()
	if status < vi.SUCCESS {
		return errors.New("could not open a session to the VISA Resource Manager")
	}
	r.ResourceManager = rm

	instr, status := rm.Open(connStr, vi.NULL, vi.NULL)
	if status < vi.SUCCESS {
		return fmt.Errorf("an error occurred opening the session to %s", connStr)
	}
	r.Instr = instr

	return nil
}

func (r *Rigol) Close() {
	r.Instr.Close()
	r.ResourceManager.Close()
}

func (r *Rigol) Write(msg string) error {
	b := []byte(msg)
	_, status := r.Instr.Write(b, uint32(len(b)))
	if status < vi.SUCCESS {
		return fmt.Errorf("error writing to the device: %v", status)
	}
	return nil
}

func (r *Rigol) Read(bytes uint32) ([]byte, error) {
	b, _, status := r.Instr.Read(bytes)
	if status < vi.SUCCESS {
		return nil, fmt.Errorf("read failed with error code %x", status)
	}
	return b, nil
}

func (r *Rigol) FetchWaveformData(source string) ([]byte, []byte, error) {
	setup := []string{
		fmt.Sprintf(":WAV:SOUR %s", source), // waveform source
		":WAV:MODE RAW",                     // capture all samples from memory, not just on screen
		":WAV:FORM BYTE",                    // data format bytes
		":WAV:STAR 1",                       // start at sample 1
		":WAV:STOP 125000",                  // capture 125k samples (max per call)
		":WAV:DATA?",                        // fetch data
	}
	for _, cmd := range setup {
		if err := r.Write(cmd); err != nil {
			return nil, nil, err
		}
	}
	d, err := r.Read(125000)
	if err != nil {
		return nil, nil, err
	}
	// header, data, error
	return d[0:11], d[11 : len(d)-1], nil
}

func (r *Rigol) Trigger() error {
	setup := []string{
		":CHAN1:DISP ON",        // Turn on ch1
		":CHAN1:PROB 10",        // 10x probe
		":CHAN1:UNIT VOLT",      // units in volts
		":CHAN1:SCAL 1",         // 1v per division
		":CHAN1:OFFS 0",         // 0 offset
		":CHAN2:DISP OFF",       // Turn off ch2
		":CHAN3:DISP OFF",       // Turn off ch3
		":CHAN4:DISP OFF",       // Turn off ch4
		":LA:STAT ON",           // Turn on the LA
		":LA:POD1:DISP ON",      // turn D0-D7 on
		":LA:POD1:THR 3",        // POD1 threshold for logic 1 at 3v
		":LA:POD2:DISP OFF",     // turn D8-D15 off
		":LA:POD2:THR 3",        // POD1 threshold for logic 1 at 3v
		":TRIG:MODE EDGE",       // trigger mode to edge
		":TRIG:EDG:SOUR CHAN1",  // trigger on Channel 1
		":TRIG:EDG:SLOP POS",    // trigger on rising edge
		":TRIG:EDG:LEV 3",       // trigger level set to 3v
		":ACQ:MDEP 125000",      // Memory depth, max is 6000000 pts when 16 LA channels enabled
		":TIM:MAIN:SCAL 0.0002", // Timebase scale in seconds
		":ACQ:TYPE HRES",        // High resolution mode
		":SING",                 // single shot wait for trigger
	}
	for _, cmd := range setup {
		if err := r.Write(cmd); err != nil {
			return err
		}
	}
	return nil
}

func (r *Rigol) WaitForCapture() error {
	for i := 0; i < 60; i++ {
		time.Sleep(1 * time.Second)

		if err := r.Write("TRIG:STAT?"); err != nil {
			return err
		}
		d, err := r.Read(100)
		if err != nil {
			return err
		}
		state := strings.Split(string(d), "\n")[0]
		if state == "STOP" {
			return nil
		}
	}
	return errors.New("timeout waiting for trigger")
}

type Preamble struct {
	Format     int64   // 0 byte, 1 word, 2 asc
	Type       int64   // 0 normal, 1 max, 2 raw
	Points     int64   // number of points
	Count      int64   // the number of averages in the average sample mode and 1 in other modes
	Xincrement float64 // time diff between points
	Xorigin    float64 // start time of waveform
	Xref       int64   // Reference time of data point
	Yincrement float64 // waveform increment in Y
	Yorigin    int64   // vertical offset
	Yref       int64   // vertical reference position
}

func (r *Rigol) FetchPreamble() (*Preamble, error) {
	err := r.Write(":WAV:PRE?")
	if err != nil {
		return nil, err
	}
	preamble, err := r.Read(100)
	if err != nil {
		return nil, err
	}
	preambleStr := strings.Split(string(preamble), "\n")[0]
	fmt.Printf("Raw Preamble: %s\n", preambleStr)
	p := &Preamble{}
	parts := strings.Split(preambleStr, ",")
	if pf, err := strconv.ParseInt(parts[0], 10, 64); err != nil {
		return nil, err
	} else {
		p.Format = pf
	}
	if pt, err := strconv.ParseInt(parts[1], 10, 64); err != nil {
		return nil, err
	} else {
		p.Type = pt
	}
	if pp, err := strconv.ParseInt(parts[2], 10, 64); err != nil {
		return nil, err
	} else {
		p.Points = pp
	}
	if pc, err := strconv.ParseInt(parts[3], 10, 64); err != nil {
		return nil, err
	} else {
		p.Count = pc
	}
	if pxi, err := strconv.ParseFloat(parts[4], 64); err != nil {
		return nil, err
	} else {
		p.Xincrement = pxi
	}
	if pxo, err := strconv.ParseFloat(parts[5], 64); err != nil {
		return nil, err
	} else {
		p.Xorigin = pxo
	}
	if pxr, err := strconv.ParseInt(parts[6], 10, 64); err != nil {
		return nil, err
	} else {
		p.Xref = pxr
	}
	if pyi, err := strconv.ParseFloat(parts[7], 64); err != nil {
		return nil, err
	} else {
		p.Yincrement = pyi
	}
	if pyo, err := strconv.ParseInt(parts[8], 10, 64); err != nil {
		return nil, err
	} else {
		p.Yorigin = pyo
	}
	if pyr, err := strconv.ParseInt(parts[9], 10, 64); err != nil {
		return nil, err
	} else {
		p.Yref = pyr
	}

	return p, nil
}

func main() {
	r := Rigol{}
	log.Println("Initializing...")
	err := r.Init("TCPIP::192.168.1.70::INSTR")
	if err != nil {
		log.Fatal(err)
	}
	defer r.Close()

	log.Println("Setting parameters and triggering...")
	err = r.Trigger()
	if err != nil {
		log.Fatal(err)
	}

	log.Println("Waiting for trigger...")
	err = r.WaitForCapture()
	if err != nil {
		log.Fatal(err)
	}

	log.Println("Trigger detected, fetching waveform data...")
	header, data, err := r.FetchWaveformData("D0") // D0 for bottom 8 bits, D8 for upper
	if err != nil {
		log.Fatal(err)
	}

	preamble, err := r.FetchPreamble()
	if err != nil {
		log.Fatal(err)
	}
	fmt.Printf("Points: %d\n", preamble.Points)
	fmt.Printf("Xincrement: %.9f\n", preamble.Xincrement)

	fmt.Println("Header:")
	for _, b := range header {
		fmt.Printf("%x ", b)
	}
	fmt.Printf("\nData: (%d samples)\n", len(data))

	/*
		D0 RD ULA 3
		D1 MREQ ULA 38
		D2 A15 ULA 37
		D3 A14 ULA 36
		D4 ROMCS ULA 34
	*/

	// find edges
	pins := map[string]int{
		"RD":    0,
		"MREQ":  1,
		"A15":   2,
		"A14":   3,
		"ROMCS": 4,
	}
	transitions := make(map[int64]byte)  // map of timestamp to pin values
	signalChanges := make(map[int]int64) // map of pin index to timestamp of change
	var timestamp int64 = 0
	var last byte = 0x00
	transitions[0] = 0x00
	for _, b := range data {
		if b != last {
			// store the raw pin values at this timestamp
			transitions[timestamp] = b
			// store whether a specific pin changed at this time
			for i := 0; i < len(pins); i++ {
				if b&1<<i != last&1<<i {
					signalChanges[i] = timestamp
				}
			}
		}
		last = b
		timestamp++
	}

	// render output
}
