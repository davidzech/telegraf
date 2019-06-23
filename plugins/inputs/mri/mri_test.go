package mri

import (
	"os"
	"sort"
	"testing"

	"github.com/stretchr/testify/assert"
)

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

	expected := map[string]interface{}{
		"ColdheadRuO": 4.19,
		"H20_Flow":    12.98,
		"H20_Temp":    30.545,
		"HDC":         21,
		"HeLvl":       68.344,
		"HePress":     1.079,
		"ReconSi410":  3.504,
		"ReconRuO":    4.215,
		"Shield":      39.222,
	}

	assert.Equal(t, expected, out)
}
