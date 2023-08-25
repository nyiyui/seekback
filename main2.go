package main

import "github.com/gordonklaus/portaudio"

func main() {
	//portaudio.Initialize()
	//defer portaudio.Terminate()
	//in := make([]int32, 64)
	//stream, err := portaudio.OpenDefaultStream(1, 0, 44100, len(in), in)
	//chk(err)
	//defer stream.Close()

	chk(portaudio.Initialize())
	defer portaudio.Terminate()
	in := make([]int32, 64)
	stream, err := portaudio.OpenDefaultStream(1, 0, 44100, len(in), in)
	chk(err)
	defer chk(stream.Close())
}
func chk(err error) {
	if err != nil {
		panic(err)
	}
}
func chk2(err error) {
	if err != nil {
		panic("chk2: " + err.Error())
	}
}
