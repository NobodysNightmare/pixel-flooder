package main

import (
	"bufio"
	"fmt"
	"image/png"
	"net"
	"os"
	"slices"
	"strconv"
	"time"
)

type Pixel struct {
	X, Y, R, G, B, A int
}

func main() {
	if len(os.Args) < 2 {
		fmt.Println("Please at least provide a file name for a PNG!")
		os.Exit(1)
	}

	remoteAddress := "localhost:1337"
	fileName := os.Args[1]
	dx := 0
	dy := 0
	transparencyCutoff := 10

	if len(os.Args) >= 4 {
		dx, _ = strconv.Atoi(os.Args[2])
		dy, _ = strconv.Atoi(os.Args[3])
	}

	file, err := os.Open(fileName)
	handleError(err)
	defer file.Close()

	reader := bufio.NewReader(file)

	image, err := png.Decode(reader)
	handleError(err)

	fmt.Println("Loaded image with bounds", image.Bounds())

	var pixels []Pixel
	minX := image.Bounds().Min.X
	minY := image.Bounds().Min.Y

	for x := minX; x < image.Bounds().Max.X; x++ {
		for y := minY; y < image.Bounds().Max.Y; y++ {
			r, g, b, a := image.At(x, y).RGBA()
			p := Pixel{X: x - minX, Y: y - minY, R: int(r / 256), G: int(g / 256), B: int(b / 256), A: int(a / 256)}

			if p.A > transparencyCutoff {
				pixels = append(pixels, p)
			}
		}
	}

	slices.SortStableFunc(pixels, func(a, b Pixel) int {
		aScore := (a.X % 2) + (a.Y % 2)
		bScore := (b.X % 2) + (b.Y % 2)
		return aScore - bScore
	})

	fmt.Println("Prepared", len(pixels), "pixels")

	var frame []byte

	for _, p := range pixels {
		frame = append(frame, p.AsSetMessage()...)
	}

	fmt.Printf("Prepared full network frame (%d kiB)\n", len(frame)/1024)

	fmt.Println("Connecting to server...")
	connection, err := net.Dial("tcp", remoteAddress)
	handleError(err)
	defer connection.Close()

	_, err = connection.Write([]byte(fmt.Sprintf("OFFSET %d %d\n", dx, dy)))
	handleError(err)

	renderedFrames := 0
	lastWritten := 0
	thisWritten := 0
	lastCheckpoint := time.Now()
	for {
		n, err := connection.Write(frame)
		handleError(err)

		thisWritten += n

		renderedFrames++
		if renderedFrames%100 == 0 {
			thisCheckpoint := time.Now()
			fmt.Printf(
				"\rRendering at %4d FPS (%4d MiB/s)",
				int(100/thisCheckpoint.Sub(lastCheckpoint).Seconds()),
				int(float64(thisWritten-lastWritten)/thisCheckpoint.Sub(lastCheckpoint).Seconds()/float64(1024*1024)),
			)
			lastCheckpoint = thisCheckpoint
			lastWritten = thisWritten
		}
	}
}

func handleError(err error) {
	if err == nil {
		return
	}

	fmt.Println(err)
	os.Exit(1)
}

func (m Pixel) AsSetMessage() []byte {
	if m.A < 255 {
		return []byte(fmt.Sprintf("PX %d %d %02x%02x%02x%02x\n", m.X, m.Y, m.R, m.G, m.B, m.A))
	}

	return []byte(fmt.Sprintf("PX %d %d %02x%02x%02x\n", m.X, m.Y, m.R, m.G, m.B))
}
