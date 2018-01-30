package save

import (
  log "github.com/sirupsen/logrus"
  // "io" //FIXME make it generic for every type of writer
  "encoding/csv"
  "sync"
)

type Saver struct {
  channels []chan []string
  wg sync.WaitGroup
}

func NewSaver() (*Saver) {
  s := &Saver{
    channels: make([]chan []string, 1)}

  return s
}

func (s *Saver) AddNewWriter(writer *csv.Writer) (chan<- []string){
  s.wg.Add(1)

  channel := make(chan []string)
  s.channels = append(s.channels, channel)

  go writeToFile(writer, channel, &s.wg)

  return channel
}

func writeToFile(w *csv.Writer, writingChannel <-chan []string, wg *sync.WaitGroup) {
  // defer w.Close() //CHECKME is it the way to close a writer/file?
  defer wg.Done()

  for entry := range writingChannel {
    //TODO write new line inside of the file
    log.Debug("writing line: ", entry)
    w.Write(entry)
    w.Flush()
  }
}
