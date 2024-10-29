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
	"github.com/influxdata/telegraf/plugins/inputs/mri_espree/ansi"
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

	emulator := ansi.NewEmulator(port)

	emulator.Escape()
	time.Sleep(2000 * time.Millisecond)

	emulator.Enter()
	time.Sleep(2000 * time.Millisecond)

	emulator.Key('r')
	time.Sleep(2000 * time.Millisecond)

	emulator.Enter()
	time.Sleep(2000 * time.Millisecond)

	err = emulator.Parse(4000)
	if err != nil {
		return err
	}
	screen := emulator.LastScreen()

	row, err := parseDataString(screen.String())
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

func parseDataString(s string) (*row, error) {
	out := row{
		fields: make(map[string]interface{}),
	}
	var (
		heliumLevelExp  = regexp.MustCompile(`Values.+\s+(?P<level1>\d{1,2}\.\d)\%+\s+(?P<level2>\d{1,2}\.\d)\%`)
		coldHeadTempExp = regexp.MustCompile(`Cold Head\s+Sensor1:(?P<sensor1>\d{1,2}\.\d)K`)
		shieldTempExp   = regexp.MustCompile(`Shield\s+Sensor1:(?P<sensor1>\d{1,2}\.\d)K\s+Sensor2:(?P<sensor2>\d{1,2}\.\d)K`)
		magnetPowerExp  = regexp.MustCompile(`Average Power\s+:(?P<power>\d+\.\d+)W`)
		magnetPsiExp    = regexp.MustCompile(`Mag psiA\s+:(?P<psi>\d+\.\d+)\s+`)
		compresssorExp  = regexp.MustCompile(`Compressor:\s+(?P<compressor>OFF|ON)`)
	)
	heliumLevel := heliumLevelExp.FindStringSubmatch(s)
	if len(heliumLevel) != 3 {
		return nil, errors.New("failed to parse Helium Level1")
	}
	heliumLevel1, heliumLevel2 := heliumLevel[heliumLevelExp.SubexpIndex("level1")], heliumLevel[heliumLevelExp.SubexpIndex("level2")]
	out.fields["helium_level1"], _ = strconv.ParseFloat(heliumLevel1, 64)
	out.fields["helium_level2"], _ = strconv.ParseFloat(heliumLevel2, 64)

	coldHeadTemp := coldHeadTempExp.FindStringSubmatch(s)
	if len(coldHeadTemp) != 2 {
		return nil, errors.New("failed to parse Cold Head temperature")
	}
	out.fields["coldhead_temperature"], _ = strconv.ParseFloat(coldHeadTemp[coldHeadTempExp.SubexpIndex("sensor1")], 64)

	// only care about sensor 1?
	shield := shieldTempExp.FindStringSubmatch(s)
	if len(shield) != 3 {
		return nil, errors.New("failed to parse shield temperature")
	}
	out.fields["shield_temperature"], _ = strconv.ParseFloat(shield[shieldTempExp.SubexpIndex("sensor1")], 64)

	magnetPower := magnetPowerExp.FindStringSubmatch(s)
	if len(magnetPower) != 2 {
		return nil, errors.New("failed to parse magnet power")
	}
	out.fields["magnet_power"], _ = strconv.ParseFloat(magnetPower[magnetPowerExp.SubexpIndex("power")], 64)

	magnetPsi := magnetPsiExp.FindStringSubmatch(s)
	if len(magnetPower) != 2 {
		return nil, errors.New("failed to parse magnet psi")
	}
	out.fields["magnet_psi"], _ = strconv.ParseFloat(magnetPsi[magnetPsiExp.SubexpIndex("psi")], 64)

	compressor := compresssorExp.FindStringSubmatch(s)
	if len(compressor) != 2 {
		return nil, errors.New("failed to parse compressor status")
	}
	switch compressor[compresssorExp.SubexpIndex("compressor")] {
	case "ON":
		out.fields["compressor"] = true
	case "OFF":
		out.fields["compressor"] = false
	default:
		return nil, errors.New("unknown compressor status")
	}

	return &out, nil
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
