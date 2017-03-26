package main

import (
	"fmt"
	"html/template"
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

func processError(w http.ResponseWriter, err error, message, page string) {
	fmt.Printf(message, "%v", err)

	t, _ := template.ParseFiles(tmplDir + page)
	t.Execute(w, &MosaicData{Error: err})
}

func processForm(w http.ResponseWriter, r *http.Request) (int, error) {
	if err := r.ParseMultipartForm(sizeLimit); nil != err {
		return 0, err
	}

	// number of tiles along the edge
	tilesCount, err := strconv.Atoi(r.FormValue("tiles"))
	if err != nil {
		return 0, err
	}

	fmt.Printf("tilesCount: %v=d", tilesCount)

	return tilesCount, nil
}

// handler to process uploaded file to create a mosaic
// and show bothe the original and the mosaic
func showMosaic(w http.ResponseWriter, r *http.Request) {
	fmt.Printf(">>> in showMosaic\n")

	tilesCount, err := processForm(w, r)
	if err != nil {
		processError(w, err, "Cannot show the mosaic page:", "upload.html")
		return
	}

	origImage, origImageEnc, err := EncodeOrigImage(r)
	if err != nil {
		processError(w, err, "Cannot encode uploaded image:", "upload.html")
		return
	}

	// create the mosaic
	mosaicEnc, err := CreateMosaic(tilesDir, origImage, tilesCount)
	if err != nil {
		processError(w, err, "Cannot show the mosaic page:", "upload.html")
		return
	}

	// prepare and process the template
	t, err := template.ParseFiles(tmplDir + "mosaic.html")
	if err != nil {
		processError(w, err, "Cannot show the mosaic page:", "upload.html")
		return
	}

	err = t.Execute(w, &MosaicData{ImageEnc: origImageEnc, MosaicEnc: mosaicEnc})
	if err != nil {
		processError(w, err, "Cannot show the mosaic page:", "upload.html")
		return
	}
}

func main() {

	http.HandleFunc("/", showUploadPage)
	http.HandleFunc("/mosaic", showMosaic)

	fmt.Printf(">>> starting server\n")
	http.ListenAndServe(":9000", nil)
}
