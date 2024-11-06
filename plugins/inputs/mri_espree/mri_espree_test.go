package mri_espree

import (
	"bytes"
	_ "embed"
	"encoding/base64"
	"testing"

	"github.com/influxdata/telegraf/plugins/inputs/mri_espree/ansi"
	"github.com/stretchr/testify/assert"
)

//go:embed testdata/putty.log
var putty []byte

func TestParseData(t *testing.T) {
	t.SkipNow()
	reader := bytes.NewBuffer(putty)

	row, err := parseData(reader)

	assert.NoError(t, err)
	assert.NotNil(t, row)
}

func TestAnsiTerm(t *testing.T) {
	assert := assert.New(t)
	reader := bytes.NewBuffer(putty)

	emulator := ansi.NewEmulator(reader)

	err := emulator.Parse(4000)
	assert.NoError(err)

	screen := emulator.LastScreen()
	row, err := parseDataString(screen.String())

	assert.NoError(err)
	assert.NotNil(row)
}

//go:embed testdata/failure.log
var failure1 string

//go:embed testdata/failure2.log
var failure2 string

//go:embed testdata/failure3.log
var failure3 string

func TestFailure1(t *testing.T) {

	cases := map[string]string{
		"failure1": failure1,
		"failure2": failure2,
		"failure3": failure3,
	}

	for name, failure := range cases {
		t.Run(name, func(t *testing.T) {
			assert := assert.New(t)
			data, _ := base64.StdEncoding.DecodeString(failure)
			reader := bytes.NewBuffer(data)
			// os.Stdout.Write(data)

			emulator := ansi.NewEmulator(reader)

			err := emulator.Parse(6000)
			assert.NoError(err)

			screen := emulator.LastScreen()
			row, err := parseDataString(screen.String())
			print(screen.String())

			assert.NoError(err)
			assert.NotNil(row)
		})
	}
}
