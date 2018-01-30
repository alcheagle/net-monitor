package main

import (
  "github.com/urfave/cli"
  log "github.com/sirupsen/logrus"
  "github.com/spf13/viper"
  "strings"
  "os"
  "time"
  "sync"
  "github.com/alcheagle/net-monitor/save"
  "strconv"
  "encoding/csv"
  "github.com/alcheagle/net-monitor/scans"
)

var (
  pingTicker      *time.Ticker
  downloadTicker  *time.Ticker
  uploadTicker    *time.Ticker
  wg              sync.WaitGroup
  saver           *save.Saver
)

func init() {
  viper.SetDefault("download-output", "download.csv")
  viper.SetDefault("upload-output", "upload.csv")
  viper.SetDefault("ping-output", "ping.csv")
}

func main() {
  app := cli.NewApp()
  app.Usage = "export rpi metrics to docker"

  app.Flags = []cli.Flag {
    cli.StringFlag {
      Name:   "logging-level, log",
      Value:  "INFO",
      Usage:  "The logging level for this application",
      EnvVar: "LOGGING_LEVEL",
    },
    cli.Uint64Flag {
      Name:   "ping-interval, pi",
      Value:  uint64(2),
      Usage:  "The interval between ping scans in seconds",
      EnvVar: "PING_INTERVAL",
    },
    cli.Uint64Flag {
      Name:   "upload-interval, ui",
      Value:  uint64(900),
      Usage:  "The interval between upload scans in seconds",
      EnvVar: "UPLOAD_INTERVAL",
    },
    cli.Uint64Flag {
      Name:   "download-interval, di",
      Value:  uint64(900),
      Usage:  "The interval between download scans in seconds",
      EnvVar: "DOWNLOAD_INTERVAL",
    },
    cli.BoolFlag {
      Name:   "ping-disable, pd",
      Usage:  "Enable ping scans",
      EnvVar: "PING_ENABLE",
    },
    cli.BoolFlag {
      Name:   "upload-disable, ud",
      Usage:  "Enable upload scans",
      EnvVar: "UPLOAD_ENABLE",
    },
    cli.BoolFlag {
      Name:   "download-disable, dd",
      Usage:  "Enable download scans",
      EnvVar: "DOWNLOAD_ENABLE",
    },
    cli.StringFlag {
      Name:   "download-output, do",
      Value:  "download.csv",
      Usage:  "The output file for download measures",
      EnvVar: "DOWNLOAD_OUTPUT",
    },
    cli.StringFlag {
      Name:   "upload-output, uo",
      Value:  "upload.csv",
      Usage:  "The output file for upload measures",
      EnvVar: "UPLOAD_OUTPUT",
    },
    cli.StringFlag {
      Name:   "ping-output, po",
      Value:  "ping.csv",
      Usage:  "output file for ping measures",
      EnvVar: "PING_OUTPUT",
    },
    cli.StringSliceFlag {
      Name:   "additional-ping-hosts, aph",
      Usage:  "Addittional address for the ping test",
      EnvVar: "PING_ADDRESS",
    },
  }

  app.Action = func(c *cli.Context) error {
    logging_level_flag := c.String("logging-level")
    logging_level, err := log.ParseLevel(logging_level_flag)

    if strings.EqualFold(logging_level_flag, "DEBUG") {
      viper.Set("debug", true)
    }

    if err != nil {
      log.Fatalf("logging level: %s doesn't exist", logging_level_flag)
    } else {
      log.SetLevel(logging_level)
    }

    viper.Set("additional-ping-hosts", c.StringSlice("additional-ping-hosts"))
    viper.Set("ping-output", c.String("ping-output"))
    viper.Set("upload-output", c.String("upload-output"))
    viper.Set("download-output", c.String("download-output"))

    log.Debug("setting up Saver")

    saver = save.NewSaver()

    log.Debug("Setting up tickers")

    if !c.Bool("ping-disable") {
      pingDuration := time.Second*time.Duration(c.Uint64("ping-interval"))
      log.Debug("Ping interval ", pingDuration)
      pingTicker      = time.NewTicker(pingDuration)

      f, err := os.OpenFile(viper.GetString("ping-output"), os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)

      if err != nil {
        log.Fatal(err)
      }
      csvWriter := csv.NewWriter(f)
      writingChannel := saver.AddNewWriter(csvWriter)
      // writingChannel := saver.AddNewWriter(f)

      wg.Add(1)
      go pingTester(writingChannel)
      defer f.Close()
    }

    if !c.Bool("upload-disable") {
      uploadDuration := time.Second*time.Duration(c.Uint64("upload-interval"))
      log.Debug("Upload interval ", uploadDuration)
      uploadTicker    = time.NewTicker(uploadDuration)

      wg.Add(1)
      f, err := os.OpenFile(viper.GetString("upload-output"), os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)

      if err != nil {
        log.Fatal(err)
      }
      csvWriter := csv.NewWriter(f)
      writingChannel := saver.AddNewWriter(csvWriter)
      // writingChannel := saver.AddNewWriter(f)

      wg.Add(1)
      go uploadTester(writingChannel)
      defer f.Close()
    }

    if !c.Bool("download-disable") {
      downloadDuration := time.Second*time.Duration(c.Uint64("download-interval"))
      log.Debug("Download interval ", downloadDuration)
      downloadTicker  = time.NewTicker(downloadDuration)
      wg.Add(1)
      f, err := os.OpenFile(viper.GetString("download-output"), os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)

      if err != nil {
        log.Fatal(err)
      }
      csvWriter := csv.NewWriter(f)
      writingChannel := saver.AddNewWriter(csvWriter)
      // writingChannel := saver.AddNewWriter(f)

      wg.Add(1)
      go downloadTester(writingChannel)
      defer f.Close()
    }

    wg.Wait()

    return nil
  }

  app.Run(os.Args)
}

func pingTester(writingChannel chan<- []string) {
  defer wg.Done()
  for t := range pingTicker.C {
    log.Debugf("new ping tick %s", t)

    results := scans.ScanPing()
    for _, result := range results {
      log.Info("Ping Scan Result: ", result)

      writingChannel <- []string{
        t.String(),
        strconv.FormatFloat(result.Rtt.Seconds()*1000, 'f', 4, 64),
        result.Addr.String()}
    }
  }
}

func uploadTester(writingChannel chan<- []string) {
  defer wg.Done()
  for t := range uploadTicker.C {
    log.Debugf("new upload tick %s", t)

    result := scans.ScanUpload()
    log.Info("Upload Scan Result: ", result)

    writingChannel <- []string{
      t.String(),
      strconv.FormatFloat(result, 'f', 4, 64),
      scans.GetTestServer().URL}
    //TODO save results of the test in a csv file
  }
}

func downloadTester(writingChannel chan<- []string) {
  defer wg.Done()
  for t := range downloadTicker.C {
    log.Debugf("new download tick %s", t)

    result := scans.ScanDownload()
    log.Info("Download Scan Result: ", result)

    writingChannel <- []string{
      t.String(),
      strconv.FormatFloat(result, 'f', 4, 64),
      scans.GetTestServer().URL}
    //TODO save results of the test in a csv file
  }
}
