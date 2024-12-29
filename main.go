package main

import (
	"bufio"
	"fmt"
	"image/png"
	"net"
	"os"
	"slices"
	"strconv"
	"strings"
	"time"

	"github.com/urfave/cli/v2"
)

type Pixel struct {
	X, Y, R, G, B, A int
}

type Offset struct {
	X, Y int
}

type Config struct {
	FileName           string
	RemoteAddress      string
	Offsets            []Offset
	TransparencyCutoff int
}

var config Config

func main() {
	app := &cli.App{
		Name:  "boom",
		Usage: "make an explosive entrance",
		Flags: []cli.Flag{
			&cli.StringFlag{
				Name:  "image",
				Usage: "Path to the PNG image that shall be used",
			},
			&cli.StringSliceFlag{
				Name:  "offset",
				Usage: "Pass offsets that should be rendered in format XxY",
			},
		},
		Action: func(cCtx *cli.Context) error {
			config.RemoteAddress = "wall.c3pixelflut.de:1337"
			config.FileName = cCtx.String("image")
			config.TransparencyCutoff = 10

			if len(cCtx.StringSlice("offset")) == 0 {
				config.Offsets = append(config.Offsets, Offset{0, 0})
			} else {
				for _, offset := range cCtx.StringSlice("offset") {
					values := strings.Split(offset, "x") // TODO: why does , not work?
					x, _ := strconv.Atoi(values[0])
					y, _ := strconv.Atoi(values[1])
					config.Offsets = append(config.Offsets, Offset{x, y})
				}
			}

			if len(config.FileName) == 0 {
				fmt.Println("Please at least provide a file name for a PNG using --image!")
				os.Exit(1)
			}

			render()

			return nil
		},
	}

	if err := app.Run(os.Args); err != nil {
		fmt.Println(err)
	}
}

func render() {
	file, err := os.Open(config.FileName)
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

			if p.A > config.TransparencyCutoff {
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
	connection, err := net.Dial("tcp", config.RemoteAddress)
	handleError(err)
	defer connection.Close()

	renderedFrames := 0
	lastWritten := 0
	thisWritten := 0
	lastCheckpoint := time.Now()
	for {
		n, err := connection.Write(config.Offsets[renderedFrames%len(config.Offsets)].AsMessage())
		handleError(err)
		thisWritten += n

		n, err = connection.Write(frame)
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

func (o Offset) AsMessage() []byte {
	return []byte(fmt.Sprintf("OFFSET %d %d\n", o.X, o.Y))
}
