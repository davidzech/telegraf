package mri_espree

import (
	"bytes"
	_ "embed"
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
