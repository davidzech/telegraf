package mri

import (
	"bufio"
	"errors"
	"fmt"
	"io"
	"os"
	"sort"
	"strconv"
	"strings"
	"time"

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
		return nil, err
	}
	err = sc.Login(m.Username, m.Password)
	if err != nil {
		return nil, err
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

func parseDate(date, t string, loc *time.Location) time.Time {
	day := parseInt(date[0:2])
	month := time.Month(parseInt(date[2:4]))
	year := parseInt(date[4:6]) + 2000

	hour := parseInt(t[0:2])
	minute := parseInt(t[2:4])

	return time.Date(year, month, day, hour, minute, 0, 0, loc)
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

type row struct {
	fields map[string]interface{}
	time   time.Time
}

func parseData(data io.Reader) (out []row) {
	scanner := bufio.NewScanner(data)
	for scanner.Scan() {
		line := scanner.Text()
		fields := strings.Fields(line)

		out = append(out,
			row{
				fields: map[string]interface{}{
					"HeLvl":       parseFloat(fields[3]),
					"H20_Flow":    parseFloat(fields[4]),
					"H20_Temp":    parseFloat(fields[6]),
					"Shield":      parseFloat(fields[8]),
					"ReconRuO":    parseFloat(fields[9]),
					"ReconSi410":  parseFloat(fields[10]),
					"ColdheadRuO": parseFloat(fields[13]),
					"HePress":     parseFloat(fields[14]),
					"HDC":         parseInt(fields[26]),
				},
				time: parseDate(fields[0], fields[1], time.Now().Location()),
			})
	}

	fmt.Printf("Gathered %d parsed fields\n", len(out))
	return
}

func (m *Mri) gatherStats() ([]row, error) {
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

	fmt.Printf("Parsing file: %v\n", latestDat)

	rsp, err := m.ftpClient.Retr(fmt.Sprintf("/CFDISK/mindata/%s", latestDat))
	if err != nil {
		return nil, err
	}

	defer rsp.Close()

	return parseData(rsp), nil
}

func (m *Mri) Gather(acc telegraf.Accumulator) error {
	if m.ftpClient == nil  || m.ftpClient.NoOp() != nil {
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

	for _, r := range fields {
		acc.AddFields("mridata", r.fields, tags, r.time)
	}

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
