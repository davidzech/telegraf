package mri2

import (
	"net/url"
	"os"
	"sort"
	"strings"
	"testing"
	"time"

	"github.com/influxdata/toml"
	"github.com/stretchr/testify/assert"
)

func TestConfiguration(t *testing.T) {
	var mri Mri2

	sampleConfig := `
CollectorName = "Collector1"
Machines = [
	{ Name = "Server1", URL = "ftp://server1.com" }, 
	{ Name = "Server2", URL = "ftp://overrideUser:overridePassword@server2.com" }
]
DefaultUsername = "user"
DefaultPassword = "daki123"
`

	sampleConfig = strings.ReplaceAll(sampleConfig, "#", "")

	t.Log(sampleConfig)
	assert.NoError(t, toml.Unmarshal([]byte(sampleConfig), &mri))
	assert.Len(t, mri.Machines, 2)
	url, err := url.Parse(mri.Machines[1].URL)
	assert.NoError(t, err)
	assert.Equal(t, url.User.String(), "overrideUser:overridePassword")
}

func TestFilterDates(t *testing.T) {
	entries := []string{
		"day290519.dat",
		"day300519.dat",
		"day310519.dat",
		"lastsaved.dat",
		"lastsent.dat",
	}
	expected := []string{
		"day290519.dat",
		"day300519.dat",
		"day310519.dat",
	}

	actual := filterDates(entries)

	assert.Equal(t, expected, actual)
}

func TestSortByDate(t *testing.T) {
	entries := []string{
		"day290519.dat",
		"day300519.dat",
		"day010120.dat",
		"day310519.dat",
	}
	expected := []string{
		"day010120.dat",
		"day310519.dat",
		"day300519.dat",
		"day290519.dat",
	}

	sort.Sort(byDate(entries))
	assert.Equal(t, expected, entries)
}

func TestParseData(t *testing.T) {
	f, _ := os.Open("testdata/day310519.dat")

	out := parseData(f)

	expected := []row{
		row{
			fields: map[string]interface{}{
				"ColdheadRuO": 4.24,
				"H20_Flow":    12.878,
				"H20_Temp":    30.381,
				"HDC":         21,
				"HeLvl":       68.344,
				"HePress":     1.083,
				"ReconRuO":    4.215,
				"ReconSi410":  3.504,
				"Shield":      39.222,
			},
			time: parseDate("310519", "2358", time.Now().Location()),
		},
		row{
			fields: map[string]interface{}{
				"ColdheadRuO": 4.19,
				"H20_Flow":    12.98,
				"H20_Temp":    30.545,
				"HDC":         21,
				"HeLvl":       68.344,
				"HePress":     1.079,
				"ReconSi410":  3.504,
				"ReconRuO":    4.215,
				"Shield":      39.222,
			},
			time: parseDate("310519", "2359", time.Now().Location()),
		},
	}

	assert.Equal(t, expected, out)
}

func TestParseDate(t *testing.T) {
	dateStr := "230619"
	timeStr := "1026"

	location, err := time.LoadLocation("America/Los_Angeles")
	if err != nil {
		panic(err)
	}

	actual := parseDate(dateStr, timeStr, location)
	expected := time.Unix(1561310760, 0)
	assert.Equal(t, expected.UTC(), actual.UTC())
}
