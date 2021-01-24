package main

import (
	"bufio"
	"encoding/binary"
	"fmt"
	"log"
	"math"
	"os"
	"os/exec"
	"os/signal"
	"strings"

	"github.com/gordonklaus/portaudio"
)

func main() {
	fileName := ""
	endlessmode := false

	if len(os.Args) < 2 {
		fileName = "Unnamed Recording"
		endlessmode = true
	} else {
		fileName = os.Args[1]
	}

	nRecordedFiles := 0

	if endlessmode {
		fileName = fmt.Sprint(fileName, nRecordedFiles, ".aiff")
	}

	if !strings.HasSuffix(fileName, ".aiff") {
		fileName += ".aiff"
	}

	fmt.Println("Recording.  Press q to stop.")

	ch := make(chan string)
	go func(ch chan string) {
		reader := bufio.NewReader(os.Stdin)
		for {
			s, err := reader.ReadString('\n')
			if err != nil {
				close(ch)
				return
			}
			ch <- s
		}
		close(ch)
	}(ch)

	sig := make(chan os.Signal, 1)
	signal.Notify(sig, os.Interrupt, os.Kill)

	f := startNewRecording(fileName)
	nSamples := 0

	portaudio.Initialize()

	in := make([]int32, 64)
	stream, err := portaudio.OpenDefaultStream(1, 0, 44100, len(in), in)
	chk(err)

	chk(stream.Start())

	for {
		select {
		case stdin := <-ch:
			if stdin == "q\n" {

				stream.Close()
				portaudio.Terminate()
				CloseRecording(f, nSamples)

				encode(fileName)
				return
			}
		default:
			chk(stream.Read())
			chk(binary.Write(f, binary.BigEndian, in))

			// Start: detect silence after 5 seconds of recording
			if (nSamples / 44100) > 5 {
				if steamIsSilent(in) {
					CloseRecording(f, nSamples)
					encode(fileName)

					if endlessmode {
						nRecordedFiles++
						fileName = fmt.Sprint("Unnamed Recording", nRecordedFiles, ".aiff")
						f = startNewRecording(fileName)
						nSamples = 0
					}
				}
			}
			// End: Determine Volume

			nSamples += len(in)
			select {
			case <-sig:
				return
			default:
			}
		}
	}

	chk(stream.Stop())
}

func startNewRecording(fileName string) *os.File {
	f, err := os.Create(fileName)
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

	return f
}

// CloseRecording is run when file is closed
func CloseRecording(f *os.File, nSamples int) {
	// fill in missing sizes
	totalBytes := 4 + 8 + 18 + 8 + 8 + 4*nSamples
	_, err := f.Seek(4, 0)
	chk(err)
	chk(binary.Write(f, binary.BigEndian, int32(totalBytes)))
	_, err = f.Seek(22, 0)
	chk(err)
	chk(binary.Write(f, binary.BigEndian, int32(nSamples)))
	_, err = f.Seek(42, 0)
	chk(err)
	chk(binary.Write(f, binary.BigEndian, int32(4*nSamples+8)))
	chk(f.Close())
}

func steamIsSilent(in []int32) bool {
	bufLength := float64(len(in))
	sum := float64(0)
	for _, n := range in {
		x := math.Abs(float64(n) / math.MaxInt32)
		sum += math.Pow(math.Min(float64(x)/0.1, 1), 2)
	}
	rms := math.Sqrt(sum / bufLength)
	return (rms < .0001)
}

func encode(fileName string) {
	artist := "Unknown Artist"
	title := "Unknown Title"

	if strings.Index(fileName, " - ") > 1 {
		spl := strings.Split(strings.Replace(fileName, ".aiff", "", 1), " - ")
		if len(spl) > 1 {
			artist = spl[0]
			title = spl[1]
		}
	}

	fmt.Println("[Encoding] ", artist, title)

	_, err := exec.Command("lame", fileName, "-b 192", "--ta", ``+artist, "--tt", ``+title).Output()
	if err != nil {
		log.Fatal(err)
	}

	e := os.Remove(fileName)
	if e != nil {
		log.Fatal(e)
	}
}

func chk(err error) {
	if err != nil {
		panic(err)
	}
}
