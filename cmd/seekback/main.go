package main

import (
	"encoding/binary"
	"flag"
	"fmt"
	"log"
	"os"
	"os/signal"
	"strconv"
	"strings"
	"syscall"
	"time"

	"github.com/gordonklaus/portaudio"
	"nyiyui.ca/seekback/notify"
)

var bufferSize int
var fileNameTemplate string
var latestFilename string

func main() {
	flag.IntVar(&bufferSize, "buffer-size", 20*10000, "buffer size to keep (i.e. audio saved before the dump command) (generally 1000 = 1 second)")
	flag.StringVar(&fileNameTemplate, "name", "seekback-%s.aiff", "filename template to save recordings to")
	flag.StringVar(&latestFilename, "latest-name", "seekback-latest.aiff", "filename to put a symlink to the latest recording")
	flag.Parse()
	if bufferSize < 0 {
		log.Fatalf("buffer size must be positive")
	}
	if strings.Count(fileNameTemplate, "%s") != 1 {
		log.Fatalf("filename template %s must contains one %%s", strconv.Quote(fileNameTemplate))
	}

	dumpCh := make(chan DumpRequest)
	go readRequests(dumpCh)
	loop(dumpCh)
}

func check(err error) {
	if err != nil {
		panic(err)
	}
}

type DumpRequest struct {
	Duration time.Duration
}

func readRequests(dumpCh chan<- DumpRequest) {
	sigs := make(chan os.Signal, 1)
	signal.Notify(sigs, syscall.SIGUSR1)
	for range sigs {
		log.Print("dumping")
		dumpCh <- DumpRequest{
			Duration: 0,
		}
	}
}

func loop(dumpCh <-chan DumpRequest) {
	// portaudio usage copied from https://github.com/gordonklaus/portaudio/blob/aafa478834f5b0f2ca23dd182b2df227935cb64b/examples/record.go
	// TODO: maybe make the stream a channel?
	notify.Notify("READY=1")
	check(portaudio.Initialize())
	defer func() {
		check(portaudio.Terminate())
	}()
	in := make([]int32, 64)
	stream, err := portaudio.OpenDefaultStream(1, 0, 44100, len(in), in)
	check(err)
	defer stream.Close()

	check(stream.Start())
	buffer := make([][64]int32, bufferSize)
	bufferIndex := 0
	bufferStart := 0
	wrapped := false
	for {
		select {
		case r := <-dumpCh:
			now := time.Now()
			log.Printf("start dump at %s", now)
			t := time.NewTimer(r.Duration)
			dump(now, stream, in, buffer[:], bufferStart, bufferIndex, t.C)
		default:
			check(stream.Read())
			buffer[bufferIndex] = [64]int32(in)
			bufferIndex++
			if bufferIndex == len(buffer) {
				wrapped = true
			}
			bufferIndex %= len(buffer)
			if wrapped {
				bufferStart++
				bufferStart %= len(buffer)
			}
		}
	}
	check(stream.Stop())
}

// dataStart and dataEnd are both inclusive.
func dump[E any](t time.Time, stream *portaudio.Stream, in []int32, data [][64]int32, dataStart, dataEnd int, stop <-chan E) {
	// TODO: Right now, audio until a message to stop sent is not saved to the buffer. Do we want to have all audio in the buffer?
	// basically copied from https://github.com/gordonklaus/portaudio/blob/aafa478834f5b0f2ca23dd182b2df227935cb64b/examples/record.go
	name := fmt.Sprintf(fileNameTemplate, t.Format(time.RFC3339))
	f, err := os.Create(name)
	check(err)

	// form chunk
	_, err = f.WriteString("FORM")
	check(err)
	check(binary.Write(f, binary.BigEndian, int32(0))) //total bytes
	_, err = f.WriteString("AIFF")
	check(err)

	// common chunk
	_, err = f.WriteString("COMM")
	check(err)
	check(binary.Write(f, binary.BigEndian, int32(18)))                //size
	check(binary.Write(f, binary.BigEndian, int16(1)))                 //channels
	check(binary.Write(f, binary.BigEndian, int32(0)))                 //number of samples
	check(binary.Write(f, binary.BigEndian, int16(32)))                //bits per sample
	_, err = f.Write([]byte{0x40, 0x0e, 0xac, 0x44, 0, 0, 0, 0, 0, 0}) //80-bit sample rate 44100
	check(err)

	// sound chunk
	_, err = f.WriteString("SSND")
	check(err)
	check(binary.Write(f, binary.BigEndian, int32(0))) //size
	check(binary.Write(f, binary.BigEndian, int32(0))) //offset
	check(binary.Write(f, binary.BigEndian, int32(0))) //block
	nSamples := 0
	defer func() {
		// fill in missing sizes
		totalBytes := 4 + 8 + 18 + 8 + 8 + 4*nSamples
		_, err = f.Seek(4, 0)
		check(err)
		check(binary.Write(f, binary.BigEndian, int32(totalBytes)))
		_, err = f.Seek(22, 0)
		check(err)
		check(binary.Write(f, binary.BigEndian, int32(nSamples)))
		_, err = f.Seek(42, 0)
		check(err)
		check(binary.Write(f, binary.BigEndian, int32(4*nSamples+8)))
		check(f.Close())
	}()

	for i := dataStart; i <= dataEnd; i = (i + 1) % len(data) {
		check(binary.Write(f, binary.BigEndian, data[i]))
		nSamples += len(in)
	}
Record:
	for i := 0; true; i++ {
		check(stream.Read())
		check(binary.Write(f, binary.BigEndian, in))
		nSamples += len(in)
		select {
		case <-stop:
			break Record
		default:
		}
	}
	writeLatestSymlink(name)
	log.Printf("Dump complete. Saved to %s", name)
	fmt.Fprintf(os.Stdout, "done\n")
}

func writeLatestSymlink(name string) {
	// It's simpler to write remove-then-link than to try making a symlink and then checking for "file already exists" errors.
	err := os.Remove(latestFilename)
	if !os.IsNotExist(err) {
		check(err)
	}
	check(os.Symlink(name, latestFilename))
}
