package mri

import (
	"bufio"
	"bytes"
	"errors"
	"fmt"
	"io"
	"io/ioutil"
	"os"
	"sort"
	"strconv"
	"strings"

	"github.com/influxdata/telegraf"
	"github.com/influxdata/telegraf/plugins/inputs"
	"github.com/jlaffaye/ftp"
)

type Mri struct {
	Name     string
	URL      string
	Username string
	Password string

	ftpClient *ftp.ServerConn
}

func (m *Mri) SampleConfig() string {
	return `
## Config Vars go below, uncomment '##'
##
## name = Machine1
## url = ftp://server.com
## username = user
## password = daki123
`
}

func (m *Mri) Description() string {
	return "MRI sensor aggregator"
}

func (m *Mri) Init() error {
	return nil
}

func (m *Mri) initFtp() (sc *ftp.ServerConn, err error) {
	fmt.Printf("Connecting to ftp server: %v\n", m.URL)
	sc, err = ftp.Connect(m.URL)
	if err != nil {
		return
	}
	err = sc.Login(m.Username, m.Password)
	if err != nil {
		return
	}
	return
}

type byDate []string

func (s byDate) Len() int {
	return len(s)
}

func (s byDate) Swap(i, j int) {
	s[i], s[j] = s[j], s[i]
}

func (s byDate) Less(i, j int) bool {
	// is s[i] less than s[j]
	lDay, lMonth, lYear := s[i][3:5], s[i][5:7], s[i][7:9]
	rDay, rMonth, rYear := s[j][3:5], s[j][5:7], s[j][7:9]

	l := lYear + lMonth + lDay
	r := rYear + rMonth + rDay

	return strings.Compare(l, r) == 1
}

func filterDates(entries []string) (out []string) {
	for _, s := range entries {
		if strings.HasPrefix(s, "day") {
			out = append(out, s)
		}
	}
	return
}

func parseInt(s string) int {
	i, _ := strconv.Atoi(s)
	return i
}

func parseFloat(s string) float64 {
	f, _ := strconv.ParseFloat(s, 64)
	return f
}

func parseData(data io.Reader) map[string]interface{} {
	scanner := bufio.NewScanner(data)
	var out map[string]interface{}
	for scanner.Scan() {
		line := scanner.Text()
		fields := strings.Fields(line)

		out = map[string]interface{}{
			"HeLvl":       parseFloat(fields[3]),
			"H20_Flow":    parseFloat(fields[4]),
			"H20_Temp":    parseFloat(fields[6]),
			"Shield":      parseFloat(fields[8]),
			"ReconRuO":    parseFloat(fields[9]),
			"ReconSi410":  parseFloat(fields[10]),
			"ColdheadRuO": parseFloat(fields[13]),
			"HePress":     parseFloat(fields[14]),
			"HDC":         parseInt(fields[26]),
		}
	}
	fmt.Printf("Gathered parsed fields: %v\n", out)
	return out
}

func (m *Mri) gatherStats() (map[string]interface{}, error) {
	if m.ftpClient == nil {
		return nil, errors.New("fptClient is nil")
	}
	entries, err := m.ftpClient.NameList("/CFDISK/mindata")
	if err != nil {
		return nil, err
	}

	entries = filterDates(entries)
	sort.Sort(byDate(entries))

	if len(entries) == 0 {
		return nil, errors.New("no entries found")
	}

	// retrieve top file

	latestDat := entries[0]

	fmt.Printf("Parsing last line of file: %v\n", latestDat)

	rsp, err := m.ftpClient.Retr(fmt.Sprintf("/CFDISK/mindata/%s", latestDat))
	if err != nil {
		return nil, err
	}

	defer rsp.Close()

	data, err := ioutil.ReadAll(rsp)

	if err != nil {
		return nil, err
	}

	return parseData(bytes.NewBuffer(data)), nil
}

func (m *Mri) Gather(acc telegraf.Accumulator) error {
	if m.ftpClient == nil {
		var err error
		if m.ftpClient, err = m.initFtp(); err != nil {
			return err
		}
	}

	fields, err := m.gatherStats()
	if err != nil {
		return err
	}

	tags := map[string]string{
		"name": m.Name,
	}
	acc.AddFields("mridata", fields, tags)

	return nil
}

func init() {
	var name = ""
	name, _ = os.Hostname()
	inputs.Add("mri", func() telegraf.Input {
		return &Mri{
			Name: name,
		}
	})
}
