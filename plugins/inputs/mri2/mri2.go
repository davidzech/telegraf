package mri2

import (
	"bufio"
	"context"
	"errors"
	"fmt"
	"io"
	"log"
	"net/url"
	"os"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/aprice/telnet"
	"github.com/influxdata/telegraf"
	"github.com/influxdata/telegraf/internal"
	"github.com/influxdata/telegraf/plugins/inputs"
	"github.com/influxdata/telegraf/plugins/inputs/mri2/data"
	"github.com/jlaffaye/ftp"
)

type Mri2 struct {
	CollectorName    string
	Machines         []Machine
	DefaultUsername  string
	DefaultPassword  string
	SyncTime         bool
	SyncTimeInterval internal.Duration

	lastSync time.Time
}

type Machine struct {
	Name string
	URL  string
}

func (m Machine) url() *url.URL {
	if !strings.HasPrefix(m.URL, "ftp://") {
		m.URL = "ftp://" + m.URL
	}
	url, err := url.Parse(m.URL)
	if err != nil {
		return nil
	}

	u, _ := url.Parse(m.URL)
	return u
}

func (m Machine) String() string {
	var urlString string = m.URL
	if url := m.url(); url != nil {
		urlString = url.Redacted()
	}
	return fmt.Sprintf("%s;%s", m.Name, urlString)
}

func (m *Mri2) SampleConfig() string {
	return `
## Config Vars go below, uncomment '##'
##
## CollectorName = "Collector1"
## Machines = [
##	{ Name = "Server1", URL = "ftp://server1.com" }, 
##	{ Name = "Server2", URL = "ftp://overrideUser:overridePassword@server2.com" }
## ]
## DefaultUsername = "user"
## DefaultPassword = "daki123"
## SynceTime = true
## SyncTimeInterval = "12h"
`
}

func (m *Mri2) Description() string {
	return "MRI sensor aggregator V2"
}

func (m *Mri2) Init() error {
	return nil
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
	return
}

func (m *Mri2) synchronizeTime(_ context.Context, machine Machine) error {
	url := machine.url()
	host := url.Hostname()

	log.Printf("[%s] synchronizing time", machine)
	conn, err := telnet.Dial(host + ":telnet")
	if err != nil {
		log.Printf("[%s] error dialing: %v", machine, err)
		return err
	}
	defer conn.Close()
	conn.RawWrite(data.Header)
	conn.RawWrite(data.Footer)
	time.Sleep(100 * time.Millisecond)
	conn.RawWrite([]byte{0xD})
	time.Sleep(100 * time.Millisecond)
	conn.RawWrite([]byte{0xD})
	time.Sleep(500 * time.Millisecond)

	d, t := currentDateTime()

	log.Printf("[%s] Setting date to: %v", machine, d)
	conn.Write([]byte("date " + d + "\r\n"))
	time.Sleep(100 * time.Millisecond)
	log.Printf("[%s] Setting time to: %v", machine, t)
	conn.Write([]byte("time " + t + "\r\n"))

	return nil
}

func currentDateTime() (d string, t string) {
	now := time.Now()

	d = now.Format("01-02-2006")
	t = now.Format("15:04:05")

	return
}

func (m *Mri2) gatherStats(ctx context.Context, machine Machine) ([]row, error) {
	log.Printf("[%s] Connecting to FTP server", machine)

	if m.SyncTime {
		// if last sync + duration < now(), its time to resync
		if m.lastSync.Add(m.SyncTimeInterval.Duration).Before(time.Now()) {
			if err := m.synchronizeTime(ctx, machine); err != nil {
				log.Printf("[%s] Failed to synchronize time: %v", machine, err)
			}
		}
		m.lastSync = time.Now()
	}

	url := machine.url()
	username := m.DefaultUsername
	password := m.DefaultPassword

	if userInfo := url.User; userInfo != nil {
		username = userInfo.Username()
		if userInfoPassword, ok := userInfo.Password(); ok {
			password = userInfoPassword
		}
	}

	ftpClient, err := ftp.Dial(url.Host, ftp.DialWithContext(ctx))
	if err != nil {
		fmt.Printf("[%s] Failed to connect to FTP server: %v\n", machine, err)
		return nil, err
	}

	if err := ftpClient.Login(username, password); err != nil {
		fmt.Printf("[%s] Failed to login to FTP server, is the username:password correct?: %v\n", machine, err)
		return nil, err
	}

	defer ftpClient.Quit()

	entries, err := ftpClient.NameList("/CFDISK/mindata")
	if err != nil {
		fmt.Printf("[%s] Failed to list /CFDISK/mindata: %v\n", machine, err)
		return nil, err
	}

	entries = filterDates(entries)
	sort.Sort(byDate(entries))

	if len(entries) == 0 {
		log.Printf("[%s] Directory is empty", machine)
		return nil, errors.New("no entries found")
	}

	// retrieve top file
	latestDat := entries[0]

	log.Printf("[%s] Parsing file: %v", machine, latestDat)

	rsp, err := ftpClient.Retr(fmt.Sprintf("/CFDISK/mindata/%s", latestDat))
	if err != nil {
		return nil, err
	}

	defer rsp.Close()

	rows := parseData(rsp)
	log.Printf("[%s] Gathered %d parsed fields", machine, len(rows))
	return rows, nil
}

func (m *Mri2) Gather(acc telegraf.Accumulator) error {

	ctx := context.Background()

	for _, machine := range m.Machines {
		log.Printf("[%s] Gathering stats", machine)

		if _, err := url.Parse(machine.URL); err != nil {
			log.Printf("[%s] has invalid url: %v", machine.Name, err)
			continue
		}

		rows, err := m.gatherStats(ctx, machine)
		if err != nil {
			fmt.Printf("!!! Gathering Stats for %v [%v] failed: %v", machine.Name, machine.URL, err)
		}
		tags := map[string]string{
			"collectorName": m.CollectorName,
			"name":          machine.Name,
		}
		for _, row := range rows {
			acc.AddFields("mridata", row.fields, tags)
		}
	}

	// wait for writing to finish
	return nil
}

func init() {
	var name = ""
	name, _ = os.Hostname()
	inputs.Add("mri2", func() telegraf.Input {
		return &Mri2{
			CollectorName:    name,
			SyncTime:         true,
			SyncTimeInterval: internal.Duration{Duration: 12 * time.Hour},
		}
	})
}
