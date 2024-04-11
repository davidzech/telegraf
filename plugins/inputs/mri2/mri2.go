package mri2

import (
	"bufio"
	"context"
	"errors"
	"fmt"
	"io"
	"net/url"
	"os"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/aprice/telnet"
	"github.com/influxdata/telegraf"
	"github.com/influxdata/telegraf/plugins/inputs"
	"github.com/influxdata/telegraf/plugins/inputs/mri2/data"
	"github.com/jlaffaye/ftp"
)

type Mri2 struct {
	CollectorName   string
	Machines        []Machine
	DefaultUsername string
	DefaultPassword string
}

type Machine struct {
	Name string
	URL  string
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

	fmt.Printf("Gathered %d parsed fields\n", len(out))
	return
}

func (m *Mri2) synchronizeTime(ctx context.Context, url *url.URL) error {
	host := url.Hostname()

	fmt.Println("synchronizing time for", host)
	fmt.Println("dialing", host)
	conn, err := telnet.Dial(host + ":telnet")
	if err != nil {
		fmt.Println("error dialing", err)
		return err
	}
	defer conn.Close()
	fmt.Println("negotiating terminal")
	conn.RawWrite(data.Header)
	conn.RawWrite(data.Footer)
	time.Sleep(100 * time.Millisecond)
	fmt.Println("[enter] - 1")
	conn.RawWrite([]byte{0xD})
	time.Sleep(100 * time.Millisecond)
	fmt.Println("[enter] - 2")
	conn.RawWrite([]byte{0xD})
	time.Sleep(500 * time.Millisecond)

	d, t := currentDateTime()

	fmt.Println("Setting date to", d)
	conn.Write([]byte("date " + d + "\r\n"))
	time.Sleep(100 * time.Millisecond)
	fmt.Println("Setting time to", t)
	conn.Write([]byte("time " + t + "\r\n"))

	return nil
}

func currentDateTime() (d string, t string) {
	now := time.Now()

	d = now.Format("01-02-2006")
	t = now.Format("15:04:05")

	return
}

func (m *Mri2) gatherStats(ctx context.Context, url *url.URL) ([]row, error) {
	fmt.Printf("Connecting to FTP server: %v\n", url.Redacted())

	if err := m.synchronizeTime(ctx, url); err != nil {
		fmt.Println("failed to synchronize time")
	}

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
		fmt.Printf("Failed to connect to FTP server: %v\n", err)
		return nil, err
	}

	if err := ftpClient.Login(username, password); err != nil {
		fmt.Printf("Failed to login to FTP server, is the username:password correct?: %v\n", err)
		return nil, err
	}

	defer ftpClient.Quit()

	entries, err := ftpClient.NameList("/CFDISK/mindata")
	if err != nil {
		fmt.Printf("Failed to list /CFDISK/mindata: %v\n", err)
		return nil, err
	}

	entries = filterDates(entries)
	sort.Sort(byDate(entries))

	if len(entries) == 0 {
		fmt.Println("Directory is empty")
		return nil, errors.New("no entries found")
	}

	// retrieve top file
	latestDat := entries[0]

	fmt.Printf("Parsing file: %v\n", latestDat)

	rsp, err := ftpClient.Retr(fmt.Sprintf("/CFDISK/mindata/%s", latestDat))
	if err != nil {
		return nil, err
	}

	defer rsp.Close()

	return parseData(rsp), nil
}

func (m *Mri2) Gather(acc telegraf.Accumulator) error {

	ctx := context.Background()

	for _, machine := range m.Machines {
		fmt.Printf("===== Gathering stats for %v [%v] =====\n", machine.Name, machine.URL)
		if !strings.HasPrefix(machine.URL, "ftp://") {
			machine.URL = "ftp://" + machine.URL
		}
		url, err := url.Parse(machine.URL)
		if err != nil {
			fmt.Printf("invalid url: %v", err)
			return err
		}

		rows, err := m.gatherStats(ctx, url)
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
			CollectorName: name,
		}
	})
}
