package main

import (
	"bufio"
	"encoding/binary"
	"flag"
	"fmt"
	"log"
	"os"
	"strings"
	"time"

	"github.com/gordonklaus/portaudio"
	"nyiyui.ca/seekback/notify"
)

var bufferSize int
var fileNameTemplate string

func main() {
	flag.IntVar(&bufferSize, "buffer-size", 20*10000, "buffer size to keep (i.e. audio saved before the dump command) (generally 1000 = 1 second)")
	flag.StringVar(&fileNameTemplate, "name", "seekback-%s.aiff", "filename template to save recordings to")
	flag.Parse()
	if bufferSize < 0 {
		log.Fatalf("buffer size must be positive")
	}
	if strings.Count(fileNameTemplate, "%s") != 1 {
		log.Fatalf("filename template must contains one %%s")
	}

	dumpCh := make(chan DumpRequest)
	go readRequests(dumpCh)
	loop(dumpCh)
}

func chk(err error) {
	if err != nil {
		panic(err)
	}
}

type DumpRequest struct {
	Duration time.Duration
}

func readRequests(dumpCh chan<- DumpRequest) {
	r := bufio.NewReader(os.Stdin)
	for {
		_, err := r.ReadString('\n')
		chk(err)
		log.Print("dumping")
		dumpCh <- DumpRequest{
			Duration: 2 * time.Second,
		}
	}
}

func loop(dumpCh <-chan DumpRequest) {
	// TODO: maybe make the stream a channel?
	notify.Notify("READY=1")
	chk(portaudio.Initialize())
	defer func() {
		chk(portaudio.Terminate())
	}()
	in := make([]int32, 64)
	stream, err := portaudio.OpenDefaultStream(1, 0, 44100, len(in), in)
	chk(err)
	defer stream.Close()

	chk(stream.Start())
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
			chk(stream.Read())
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
	chk(stream.Stop())
}

// dataStart and dataEnd are both inclusive.
func dump[E any](t time.Time, stream *portaudio.Stream, in []int32, data [][64]int32, dataStart, dataEnd int, stop <-chan E) {
	name := fmt.Sprintf(fileNameTemplate, t.Format(time.RFC3339))
	f, err := os.Create(name)
	chk(err)

	// form chunk
	_, err = f.WriteString("FORM")
	chk(err)
	chk(binary.Write(f, binary.BigEndian, int32(0))) //total bytes
	_, err = f.WriteString("AIFF")
	chk(err)

	// common chunk
	_, err = f.WriteString("COMM")
	chk(err)
	chk(binary.Write(f, binary.BigEndian, int32(18)))                  //size
	chk(binary.Write(f, binary.BigEndian, int16(1)))                   //channels
	chk(binary.Write(f, binary.BigEndian, int32(0)))                   //number of samples
	chk(binary.Write(f, binary.BigEndian, int16(32)))                  //bits per sample
	_, err = f.Write([]byte{0x40, 0x0e, 0xac, 0x44, 0, 0, 0, 0, 0, 0}) //80-bit sample rate 44100
	chk(err)

	// sound chunk
	_, err = f.WriteString("SSND")
	chk(err)
	chk(binary.Write(f, binary.BigEndian, int32(0))) //size
	chk(binary.Write(f, binary.BigEndian, int32(0))) //offset
	chk(binary.Write(f, binary.BigEndian, int32(0))) //block
	nSamples := 0
	defer func() {
		// fill in missing sizes
		totalBytes := 4 + 8 + 18 + 8 + 8 + 4*nSamples
		_, err = f.Seek(4, 0)
		chk(err)
		chk(binary.Write(f, binary.BigEndian, int32(totalBytes)))
		_, err = f.Seek(22, 0)
		chk(err)
		chk(binary.Write(f, binary.BigEndian, int32(nSamples)))
		_, err = f.Seek(42, 0)
		chk(err)
		chk(binary.Write(f, binary.BigEndian, int32(4*nSamples+8)))
		chk(f.Close())
	}()

	for i := dataStart; i <= dataEnd; i = (i + 1) % len(data) {
		chk(binary.Write(f, binary.BigEndian, data[i]))
		nSamples += len(in)
	}
Record:
	for i := 0; true; i++ {
		chk(stream.Read())
		chk(binary.Write(f, binary.BigEndian, in))
		nSamples += len(in)
		select {
		case <-stop:
			break Record
		default:
		}
	}
	log.Printf("Dump complete. Saved to %s", name)
}
