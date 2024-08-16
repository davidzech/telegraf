package mri_espree

import (
	"errors"
	"io"
	"log"
	"os"
	"regexp"
	"strconv"
	"time"

	"github.com/influxdata/telegraf"
	"github.com/influxdata/telegraf/plugins/inputs"
	"go.bug.st/serial"
)

type Espree struct {
	Name string
	Port string // defaults to "COM1"
}

// Description implements telegraf.Input.
func (e *Espree) Description() string {
	return "Espree sensor aggregator"
}

// Gather implements telegraf.Input.
func (e *Espree) Gather(acc telegraf.Accumulator) error {
	mode := &serial.Mode{
		BaudRate: 9600,
		Parity:   0,
		DataBits: 8,
		StopBits: serial.OneStopBit,
	}

	port, err := serial.Open(e.Port, mode)
	if err != nil {
		log.Printf("[%s] Failed to open serial port %q: %v", e.Name, e.Port, err)
		return err
	}
	defer port.Close()

	escape(port)
	enter(port)
	run(port)
	enter(port)

	row, err := parseData(port)
	if err != nil {
		log.Printf("[%s] Failed to parse data from serial port %q: %v", e.Name, e.Port, err)
		return err
	}

	tags := map[string]string{
		"name": e.Name,
	}

	acc.AddFields("espree", row.fields, tags)

	return nil
}

type row struct {
	fields map[string]interface{}
	// time   time.Time
}

func parseData(r io.Reader) (*row, error) {

	var buf [2000]byte
	_, err := io.ReadFull(r, buf[:])
	if err != nil {

		return nil, err
	}

	s := string(buf[:])

	out := row{
		fields: make(map[string]interface{}),
	}

	var (
		// 	dateTimeExp            = regexp.MustCompile(`\[3;38H(\d\d:\d\d:\d\d)\s(\d\d-\w{3}-\d\d)"`)
		// 	heliumSettingsExp      = regexp.MustCompile(`\[7;17H(?P<000>\w{3})\s\s(P?<100>\d{3})\s\s(P?<alarm_l>\d{2})\s(P?<alarm_r>\d{2})\s\s(P?<warn_l>\d{2})\s(P?<warn_r>\d{2})`)
		heliumLevel1Exp        = regexp.MustCompile(`\[7;41H(.{3,5})\s`)
		heliumLevel2Exp        = regexp.MustCompile(`\[7;48H(.{3,5})\s`)
		temperatureColdheadExp = regexp.MustCompile(`\[13;20H(\d{2}\.\d)`)
		temperatureShieldExp   = regexp.MustCompile(`\[14;20H(\d{2}\.\d)`)
		magnetPowerExp         = regexp.MustCompile(`\[21;20H(\d\.\d{3})`)
		magnetPsiExp           = regexp.MustCompile(`\[20;20H(\d{2}\.\d\d)`)
		compressorExp          = regexp.MustCompile(`Compressor:.+(OFF|ON)`)
	)

	level1 := heliumLevel1Exp.FindStringSubmatch(s)
	if len(level1) == 0 {
		return nil, errors.New("failed to parse Helium Level1")
	}
	out.fields["helium_level1"], _ = strconv.ParseFloat(level1[0], 64)

	level2 := heliumLevel2Exp.FindStringSubmatch(s)
	if len(level2) == 0 {
		return nil, errors.New("failed to parse Helium Level2")
	}
	out.fields["helium_level2"], _ = strconv.ParseFloat(level2[0], 64)

	coldHead := temperatureColdheadExp.FindStringSubmatch(s)
	if len(coldHead) == 0 {
		return nil, errors.New("failed to parse Cold Head temperature")
	}
	out.fields["coldhead_temperature"], _ = strconv.ParseFloat(coldHead[0], 64)

	shield := temperatureShieldExp.FindStringSubmatch(s)
	if len(shield) == 0 {
		return nil, errors.New("failed to parse shield temperature")
	}
	out.fields["shield_temperature"], _ = strconv.ParseFloat(shield[0], 64)

	magnetPower := magnetPowerExp.FindStringSubmatch(s)
	if len(magnetPower) == 0 {
		return nil, errors.New("failed to parse magnet power")
	}
	out.fields["magnet_power"], _ = strconv.ParseFloat(magnetPower[0], 64)

	magnetPsi := magnetPsiExp.FindStringSubmatch(s)
	if len(magnetPsi) == 0 {
		return nil, errors.New("failed to parse magnet psi")
	}
	out.fields["magnet_psi"], _ = strconv.ParseFloat(magnetPsi[0], 64)

	compressor := compressorExp.FindStringSubmatch(s)
	if len(compressor) == 0 {
		return nil, errors.New("failed to parse compressor status")
	}

	compressorStatus := compressor[0]
	if compressorStatus == "OFF" {
		out.fields["compressor"] = false
	} else {
		out.fields["compressor"] = true
	}

	return &out, nil
}

func escape(port serial.Port) {
	port.Write([]byte{27})
	time.Sleep(2000 * time.Millisecond)
}

func enter(port serial.Port) {
	port.Write([]byte{13})
	time.Sleep(2000 * time.Millisecond)
}

func run(port serial.Port) {
	port.Write([]byte{114})
	time.Sleep(2000 * time.Millisecond)
}

// SampleConfig implements telegraf.Input.
func (e *Espree) SampleConfig() string {
	return `
## Config Vars go below, uncomment '##'
##
## Name = "NameOfThisMachine"
## Port = "COM1" 
`
}

func init() {
	var name = ""
	name, _ = os.Hostname()
	inputs.Add("mri_espree", func() telegraf.Input {
		return &Espree{
			Name: name,
			Port: "COM1",
		}
	})
}
