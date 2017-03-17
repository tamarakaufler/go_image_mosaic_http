package main

import (
	"bytes"
	"encoding/base64"
	"fmt"
	"html/template"
	"image"
	"image/jpeg"
	"net/http"
	"strconv"
)

var (
	tmplDir  string
	tilesDir string
)

const sizeLimit int64 = (1 << 10) * 5

type MosaicData struct {
	Error     error
	ImageEnc  string
	MosaicEnc string
}

func init() {
	tmplDir = "templates/"
	tilesDir = "images/"
}

func showUploadPage(w http.ResponseWriter, r *http.Request) {
	fmt.Printf(">>> in showUploadPage\n")

	t, err := template.ParseFiles(tmplDir + "upload.html")
	if err != nil {
		fmt.Fprintf(w, "Cannot show the photo upload page: %v", err)
		return
	}

	t.Execute(w, &MosaicData{ImageEnc: "", MosaicEnc: ""})
}

// handler to process uploaded file to create a mosaic
// and show bothe the original and the mosaic
func showMosaic(w http.ResponseWriter, r *http.Request) {
	fmt.Printf(">>> in showMosaic\n")

	if err := r.ParseMultipartForm(sizeLimit); nil != err {
		fmt.Printf("Cannot show the mosaic page: %v", err)

		t, _ := template.ParseFiles(tmplDir + "upload.html")
		t.Execute(w, &MosaicData{Error: err})
		return
	}

	// number of tiles along the edge
	tilesCount, _ := strconv.Atoi(r.FormValue("tiles"))
	fmt.Printf("tilesCount: %v=d", tilesCount)

	origImage, origImageEnc, err := encodeOrigImage(r)
	if err != nil {
		t, _ := template.ParseFiles(tmplDir + "upload.html")
		t.Execute(w, &MosaicData{Error: err})
		return
	}

	// create the mosaic
	mosaicEnc := CreateMosaic(tilesDir, origImage, tilesCount)

	// prepare and process the template
	t, err := template.ParseFiles(tmplDir + "mosaic.html")
	if err != nil {
		fmt.Printf("Cannot show the mosaic page: %v", err)

		t, _ := template.ParseFiles(tmplDir + "upload.html")
		t.Execute(w, &MosaicData{Error: err})
		return
	}

	err = t.Execute(w, &MosaicData{ImageEnc: origImageEnc, MosaicEnc: mosaicEnc})
	if err != nil {
		fmt.Printf("Cannot show the mosaic page: %v", err)

		t, _ := template.ParseFiles(tmplDir + "upload.html")
		t.Execute(w, &MosaicData{Error: err})
		return
	}
}

func encodeOrigImage(r *http.Request) (image.Image, string, error) {

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

func main() {

	http.HandleFunc("/", showUploadPage)
	http.HandleFunc("/mosaic", showMosaic)

	fmt.Printf(">>> starting server\n")
	http.ListenAndServe(":9000", nil)
}
