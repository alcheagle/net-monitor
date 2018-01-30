package scans

import (
  "github.com/zpeters/speedtest/sthttp"
	"github.com/zpeters/speedtest/tests"
  log "github.com/sirupsen/logrus"
	"github.com/spf13/viper"
	"github.com/dchest/uniuri"
	"time"
	"net/url"
	"github.com/tatsushid/go-fastping"
	"net"
	"sync"
)

type PingScan struct {
	Rtt time.Duration
	Addr *net.IPAddr
}

var (
	stClient *sthttp.Client
	tester *tests.Tester
	testServer sthttp.Server
	pinger *fastping.Pinger
	pingChannel chan PingScan

	updateServerLock sync.RWMutex

	oldIpAddr *net.IPAddr
)

func init() {
	viper.SetDefault("debug", false)
	viper.SetDefault("quiet", false)
	viper.SetDefault("report", false)
	viper.SetDefault("numclosest", 3)
	viper.SetDefault("numlatencytests", 5)
	viper.SetDefault("reportchar", "|")
	viper.SetDefault("algotype", "max")
	viper.SetDefault("httptimeout", 15)
	viper.SetDefault(
    "dlsizes",
    []int{350, 500, 750, 1000, 1500, 2000, 2500, 3000, 3500, 4000})
	viper.SetDefault(
    "ulsizes",
    []int{
			int(0.25 * 1024 * 1024),
			int(0.5 * 1024 * 1024),
			int(1.0 * 1024 * 1024),
			int(1.5 * 1024 * 1024),
			int(2.0 * 1024 * 1024)})
	viper.SetDefault(
    "speedtestconfigurl", "http://c.speedtest.net/speedtest-config.php?x="+uniuri.New())
	viper.SetDefault(
    "speedtestserversurl",
    "http://c.speedtest.net/speedtest-servers-static.php?x="+uniuri.New())
  viper.SetDefault(
    "useragent",
    "Mozilla/5.0 (Macintosh; Intel Mac OS X 10_12_0) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/55.0.2883.21 Safari/537.36")

	stClient = sthttp.NewClient( //FIXME make me global to this package
		&sthttp.SpeedtestConfig{
			ConfigURL:       viper.GetString("speedtestconfigurl"),
			ServersURL:      viper.GetString("speedtestserversurl"),
			AlgoType:        viper.GetString("algotype"),
			NumClosest:      viper.GetInt("numclosest"),
			NumLatencyTests: viper.GetInt("numlatencytests"),
			Interface:       viper.GetString("interface"),
			Blacklist:       viper.GetStringSlice("blacklist"),
			UserAgent:       viper.GetString("useragent"),
		},
		&sthttp.HTTPConfig{
			HTTPTimeout: viper.GetDuration("httptimeout") * time.Second,
		},
		viper.GetBool("debug"),
		viper.GetString("reportchar"))

	tester = tests.NewTester( //FIXME make me global to this package
		stClient,
		viper.Get("dlsizes").([]int),
		viper.Get("ulsizes").([]int),
		viper.GetBool("quiet"),
		viper.GetBool("report"))

	pingChannel = make(chan PingScan, 2)
	pinger = fastping.NewPinger()
	pinger.OnRecv = func(_addr *net.IPAddr, _rtt time.Duration) {
		log.Debugf("IP Addr: %s receive, RTT: %v", _addr.String(), _rtt)
		pingChannel <- PingScan{Rtt: _rtt, Addr: _addr}
	}

	pinger.OnIdle = func() {
		log.Debug("finish")
	}

	go UpdateServers()
}

func UpdateServers() {
	updateServerLock.Lock()

	config, err := stClient.GetConfig() //WTF????
	if err != nil {
		log.Printf("Cannot get speedtest config\n")
		log.Fatal(err)
	}
	stClient.Config = &config

	allServers, err := stClient.GetServers()
	if err != nil {
		log.Fatal(err)
	}

	closestServers := stClient.GetClosestServers(allServers)
	// log.Debug("list of closest servers ", closestServers)
	testServer = stClient.GetFastestServer(closestServers)
	log.Debug("asdasdasd")


	log.Infof(
		"Testing Server: %s - %s (%s) rtt: %f\n",
		testServer.ID,
		testServer.Name,
		testServer.Sponsor,
		testServer.Latency)

	// Setting up pinger
	hosts := viper.GetStringSlice("additional-ping-hosts")
	log.Debug("additional-ping-hosts: ", hosts)
	//Adding specified ping host
	for _, addr := range hosts {
		ra, err := net.ResolveIPAddr("ip4:icmp", addr)
		if err != nil {
			log.Fatal("unable to resolve: ", addr)
		}
		pinger.AddIPAddr(ra)
	}

	url, _ := url.Parse(testServer.URL)
	log.Debug(url.Host)

	if oldIpAddr != nil {
		pinger.RemoveIPAddr(oldIpAddr)
	}

	ra, err := net.ResolveIPAddr("ip4:icmp", url.Host)
	oldIpAddr = ra
	if err != nil {
		log.Fatal("unable to resolve: ", url.Host)
	}
	pinger.AddIPAddr(ra)
	updateServerLock.Unlock()
}

func GetTestServer() (sthttp.Server) {
	return testServer
}

func ScanPing() ([]PingScan) { //TODO use GetLatency from stClient
	updateServerLock.RLock()
	err := pinger.Run()
	updateServerLock.RUnlock()
	if err != nil {
		log.Fatal("error has occourred while running pinger: ", err)
	}

	hosts := viper.GetStringSlice("additional-ping-hosts")

	pingScans := make([]PingScan, len(hosts)+1)
	for i := 0; i < len(hosts)+1; i++  {
		pingScans[i] = <-pingChannel
		log.Debug(pingScans[i])
	}

	return pingScans
}

func ScanUpload() (float64) {
	updateServerLock.RLock()
	result := tester.Upload(testServer)
	updateServerLock.RUnlock()
	return result
}

func ScanDownload() (float64) {
	updateServerLock.RLock()
	result:= tester.Download(testServer)
	updateServerLock.RUnlock()
	return result
}
