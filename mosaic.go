package main

import (
	"bufio"
	"bytes"
	"encoding/base64"
	"errors"
	"fmt"
	"github.com/nfnt/resize"
	"image"
	"image/draw"
	"image/jpeg"
	"io/ioutil"
	"math"
	"net/http"
	"os"
	"regexp"
	"sync"
	"time"
)

type Tile interface {
	getTileColour()
}

type TileGeometry struct {
	x, y, xMin, yMin int
	wg               *sync.WaitGroup
	mutex            *sync.Mutex
}

type TileImage struct {
	filename   string
	xMin       int
	yMin       int
	scaled     image.Image
	averageRGB []float64
}

type tileMessage struct {
	filename string
	tile     *TileImage
}

func getImageColour(image image.Image, xMin, yMin, xMax, yMax int) []float64 {

	var rSum, gSum, bSum float64 = 0.0, 0.0, 0.0

	for y := yMin; y <= yMax; y++ {
		for x := xMin; x <= xMax; x++ {

			r, g, b, _ := image.At(x, y).RGBA()
			rSum += float64(r)
			gSum += float64(g)
			bSum += float64(b)
		}
	}

	pixelCount := uint32((xMax - xMin) * (yMax - yMin))
	rAvr := rSum / float64(pixelCount)
	gAvr := gSum / float64(pixelCount)
	bAvr := bSum / float64(pixelCount)

	averageRGB := []float64{rAvr, gAvr, bAvr}

	return averageRGB
}

// calculates tile photo colour
func getTileColour(wg *sync.WaitGroup, xDelta int, filename string, tileImage image.Image,
	tileData chan tileMessage) {

	resizedTile := resize.Resize(uint(xDelta), 0, tileImage, resize.Lanczos3)

	xMin := resizedTile.Bounds().Min.X
	xMax := resizedTile.Bounds().Max.X
	yMin := resizedTile.Bounds().Min.Y
	yMax := resizedTile.Bounds().Max.Y

	averageRGB := getImageColour(resizedTile, xMin, yMin, xMax, yMax)

	tile := &TileImage{
		xMin:       tileImage.Bounds().Min.X,
		yMin:       tileImage.Bounds().Min.Y,
		averageRGB: averageRGB,
		scaled:     resizedTile,
	}

	message := tileMessage{
		filename: filename,
		tile:     tile,
	}

	tileData <- message

	wg.Done()
}

func (tile TileGeometry) drawMosaicTile(tiles map[string]*TileImage, origImage image.Image,
	xDelta, yDelta int, newImage draw.Image) {

	x, y, xMin, yMin := tile.x, tile.y, tile.xMin, tile.yMin
	//wg := tile.wg
	//mutex := tile.mutex

	origRGB := getImageColour(origImage, x, y, x+xDelta, y+yDelta)

	var nearestFilename string
	var tileVectorDiff float64
	smallestDiff := 99999999.0

	for file, tile := range tiles {

		r, g, b := tile.averageRGB[0], tile.averageRGB[1], tile.averageRGB[2]

		tileVectorDiff = math.Sqrt(math.Pow((origRGB[0]-r), 2) + math.Pow((origRGB[1]-g), 2) + math.Pow((origRGB[2]-b), 2))

		if tileVectorDiff < smallestDiff {
			smallestDiff = tileVectorDiff
			nearestFilename = file
		}

	}

	// draw the tile into the new image
	tile.mutex.Lock()
	draw.Draw(newImage, image.Rectangle{image.Point{x, y}, image.Point{x + xDelta, y + yDelta}}, tiles[nearestFilename].scaled, image.Point{xMin, yMin}, draw.Src)
	tile.mutex.Unlock()

	tile.wg.Done()

}

func EncodeOrigImage(r *http.Request) (image.Image, string, error) {

	// original image
	uploaded, _, err := r.FormFile("file")
	if err != nil {
		fmt.Printf("Cannot show the mosaic page: %v", err)

		return nil, "", err
	}
	defer uploaded.Close()

	origImage, err := jpeg.Decode(uploaded)
	if err != nil {
		fmt.Printf("Cannot show the mosaic page: %v", err)

		return nil, "", err
	}

	origImageContent := new(bytes.Buffer)
	jpeg.Encode(origImageContent, origImage, nil)

	origImageEnc := base64.StdEncoding.EncodeToString(origImageContent.Bytes())

	return origImage, origImageEnc, nil
}

func CreateMosaic(tilesDir string, origImage image.Image, tilesCount int) (string, error) {

	// prepare the tiles
	tileFiles, err := ioutil.ReadDir(tilesDir)
	if err != nil {
		fmt.Printf("%s\n", err)
		return "", err
	}

	var wg sync.WaitGroup
	var mutex sync.Mutex

	var imagePattern = regexp.MustCompile(`^.*\.(jpg|JPG|jpeg|JPEG)$`)

	xMin := origImage.Bounds().Min.X
	xMax := origImage.Bounds().Max.X
	yMin := origImage.Bounds().Min.Y
	yMax := origImage.Bounds().Max.Y

	xDelta := int((xMax - xMin) / tilesCount)
	yDelta := int((yMax - yMin) / tilesCount)

	if xDelta <= 0 || yDelta <= 0 {
		errorMessage := fmt.Sprintf("ERROR: xDelta=%d, yDelta=%d, must be > 0\n\n", xDelta, yDelta)
		err := errors.New(errorMessage)
		return "", err
	}

	fmt.Printf("--> xMin=%v, xMax=%v, xDelta=%v\n\n", xMin, xMax, xDelta)
	fmt.Printf("--> yMin=%v, yMax=%v, yDelta=%v\n\n", yMin, yMax, yDelta)

	tileData := make(chan tileMessage, len(tileFiles))

	tStart := time.Now()

	// loop through files
	// goroutine to process each file
	//		create TileImage struct to hold tile data
	//		find average pixel values
	//		send to channel
	for _, fileTile := range tileFiles {
		filename := fileTile.Name()

		if !imagePattern.MatchString(filename) {
			continue
		}

		file, err := os.Open(tilesDir + filename)
		if err != nil {
			continue
		}

		// decode into an image
		tileImage, err := jpeg.Decode(bufio.NewReader(file))
		if err != nil {
			continue
		}

		wg.Add(1)

		go func(wg *sync.WaitGroup, xDelta int, filename string, tileImage image.Image,
			tileData chan tileMessage) {
			getTileColour(wg, xDelta, filename, tileImage, tileData)

		}(&wg, xDelta, filename, tileImage, tileData)

	}

	wg.Wait()

	// close the channel after processing all the tiles
	close(tileData)

	tiles := make(map[string]*TileImage)

	for m := range tileData {
		tiles[m.filename] = m.tile
	}

	fmt.Printf("Number of files = %d\n", len(tileFiles))
	fmt.Printf("Number of tiles keys = %d\n", len(tiles))

	tEnd := time.Now()

	fmt.Printf("\t==> Tile processing took %v to run.\n", tEnd.Sub(tStart))

	tStart = time.Now()

	// change into a mosaic
	//		create a new empty image of the same size as the original one
	//		on which the tiles will be placed
	newImage := image.NewRGBA(image.Rect(xMin, yMin, xMax, yMax))

	// find the tile that is nearestFilename in colour
	for y := yMin; y <= yMax; y += yDelta {
		for x := xMin; x <= xMax; x += xDelta {

			wg.Add(1)

			go func(xMin, yMin, x, y int) {
				newTile := &TileGeometry{x, y, xMin, yMin, &wg, &mutex}
				newTile.drawMosaicTile(tiles, origImage, xDelta, yDelta, newImage)
			}(xMin, yMin, x, y)
		}
	}

	wg.Wait()

	tEnd = time.Now()
	fmt.Printf("\t==> Mosaic processing took %v to create.\n", tEnd.Sub(tStart))

	// save the new finished image
	mosaicFile, err := os.Create("mosaic.jpg")
	if err != nil {
		return "", err
	}
	defer mosaicFile.Close()

	var opt jpeg.Options
	opt.Quality = 80

	// encode as base64 for display on the web page
	tileBuf := new(bytes.Buffer)
	jpeg.Encode(tileBuf, newImage, &opt)

	mosaicEnc := base64.StdEncoding.EncodeToString(tileBuf.Bytes())

	fmt.Println("THE END ...")

	return mosaicEnc, nil
}
