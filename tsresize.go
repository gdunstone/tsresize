package main

import (
	"flag"
	"fmt"
	"github.com/anthonynsimon/bild/imgio"
	"github.com/anthonynsimon/bild/transform"
	"github.com/rwcarlsen/goexif/exif"
	"golang.org/x/image/tiff"
	"image"
	"io"
	"io/ioutil"
	"os"
	"path"
	"path/filepath"
	"strconv"
	"strings"
)

var (
	rootDir          string
	outputDir        string
	targetResolution string
	targetExtension  string
	resolutionPair   []int
	imageEncoder     imgio.Encoder
)

func ERRLOG(format string, a ...interface{}) (n int, err error) {
	return fmt.Fprintf(os.Stderr, format+"\n", a...)
}

func OUTPUT(a ...interface{}) (n int, err error) {
	return fmt.Fprintln(os.Stdout, a...)
}

// TIFFEncoder returns an encoder to the Tagged Image Format
func TIFFEncoder(compressionType tiff.CompressionType) imgio.Encoder {
	return func(w io.Writer, img image.Image) error {
		return tiff.Encode(w, img, &tiff.Options{Compression: compressionType})
	}
}

func getExifData(thisFile string) ([]byte, error) {
	fileHandler, err := os.Open(thisFile)
	if err != nil {
		// file wouldnt open
		return []byte{}, err
	}

	exifData, err := exif.Decode(fileHandler)
	if err != nil {
		// exif wouldnt decode
		return []byte{}, err
	}

	jsonBytes, err := exifData.MarshalJSON()
	if err != nil {
		return []byte{}, err
	}
	return jsonBytes, nil
}

func convertImage(sourcePath, destPath string) error {
	exifJson, err := getExifData(sourcePath)

	if err != nil {
		ERRLOG("[exif] couldnt read data from %s", sourcePath)
	}
	if len(exifJson) > 0 {
		err := ioutil.WriteFile(destPath+".json", exifJson, 0644)
		if err != nil {
			ERRLOG("[exif] couldnt write json %s", destPath)
		}
	}

	img, err := imgio.Open(sourcePath)
	if err != nil {
		return err
	}
	resized := transform.Resize(img, resolutionPair[0], resolutionPair[1], transform.Lanczos)

	if err := imgio.Save(destPath, resized, imageEncoder); err != nil {
		return err
	}
	return nil
}

func visit(filePath string, info os.FileInfo, _ error) error {
	// skip directories
	if info.IsDir() {
		return nil
	}
	ext := path.Ext(filePath)
	switch extlower := strings.ToLower(ext); extlower {
	case ".jpeg", ".jpg", ".tif", ".tiff", ".cr2":
		break
	default:
		return nil
	}

	basePath := path.Base(filePath)
	// parse the new filepath
	noExtension := strings.TrimSuffix(basePath, ext)
	newBase := fmt.Sprintf("%s.%s", noExtension, targetExtension)
	newPath := path.Join(outputDir, newBase)

	// convert the image
	if err := convertImage(filePath, newPath); err != nil {
		ERRLOG("[convert] %s", err)
		return nil
	}
	// output the full image path

	if absPath, err := filepath.Abs(newPath); err == nil {
		OUTPUT(absPath)
	} else {
		OUTPUT(newPath)
	}

	return nil
}

var usage = func() {
	ERRLOG("usage of %s:\n", os.Args[0])
	//fmt.Fprintf(os.Stderr, "\tcopy into structure:\n")
	//fmt.Fprintf(os.Stderr, "\t\t %s <source>\n", os.Args[0])
	//fmt.Fprintf(os.Stderr, "\tcopy into structure at <destination>:\n")
	//fmt.Fprintf(os.Stderr, "\t\t %s <source> -output=<destination>\n", os.Args[0])
	//fmt.Fprintf(os.Stderr, "\tcopy into structure with <name> prefix:\n")
	//fmt.Fprintf(os.Stderr, "\t\t %s <source> -name=<name>\n", os.Args[0])
	//fmt.Fprintf(os.Stderr, "\trename (move) into structure:\n")
	//fmt.Fprintf(os.Stderr, "\t\t %s <source> -del\n", os.Args[0])
	//
	//fmt.Fprintln(os.Stderr, "")
	//fmt.Fprintf(os.Stderr, "flags:\n")
	//fmt.Fprintf(os.Stderr, "\t-del: removes the source files\n")
	//fmt.Fprintf(os.Stderr, "\t-name: renames the prefix fo the target files\n")

	pwd, _ := os.Getwd()
	ERRLOG("\t-type: set the output image type (default=jpeg)\n")
	ERRLOG("\t\tavailable image types:\n")
	ERRLOG("\t\tjpeg, png\n")
	ERRLOG("\t\ttiff: tiff with Deflate compression (alias for tiff-deflate)\n")
	ERRLOG("\t\ttiff-lzw: tiff with LZW compression\n")
	ERRLOG("\t\ttiff-none: tiff with no compression\n")
	ERRLOG("\t-output: set the <destination> directory (default=%s)\n", pwd)
}

func init() {
	flagset := flag.NewFlagSet("", flag.ExitOnError)

	flagset.Usage = usage
	flag.Usage = usage
	// set flags for flagset
	flagset.StringVar(&outputDir, "output", "", "output directory")
	outputType := flagset.String("type", "jpeg", "output image type")

	flagset.StringVar(&targetResolution, "res", "1920x1080", "target resolution")

	// parse the leading argument with normal flag.Parse
	flag.Parse()
	if flag.NArg() < 1 {
		ERRLOG("[path] no <source> specified")
		usage()
		os.Exit(1)
	}

	// parse flags using a flagset, ignore the first 2 (first arg is program name)
	flagset.Parse(os.Args[2:])

	rootDir = flag.Arg(0)

	switch *outputType {
	case "jpeg":
		imageEncoder = imgio.JPEGEncoder(95)
		targetExtension = "jpeg"
	case "tiff":
		imageEncoder = TIFFEncoder(tiff.Deflate)
		targetExtension = "tif"
	case "tiff-lzw":
		imageEncoder = TIFFEncoder(tiff.LZW)
		targetExtension = "tif"
	case "tiff-deflate":
		imageEncoder = TIFFEncoder(tiff.Deflate)
		targetExtension = "tif"
	case "tiff-none":
		imageEncoder = TIFFEncoder(tiff.Uncompressed)
		targetExtension = "tif"
	case "png":
		imageEncoder = imgio.PNGEncoder()
		targetExtension = "png"
	default:
		imageEncoder = imgio.JPEGEncoder(95)
		targetExtension = "jpeg"
	}
}

func main() {
	if _, err := os.Stat(rootDir); err != nil {
		if os.IsNotExist(err) {
			ERRLOG("[path] <source> %s does not exist.", rootDir)
			os.Exit(1)
		}
	}

	if outputDir == "" {
		outputDir = path.Join(rootDir, targetResolution)
		ERRLOG("[path] no <destination>, creating %s", outputDir)
	}
	os.MkdirAll(outputDir, 0755)

	ra := strings.Split(targetResolution, "x")
	for _, i := range ra[:2] {
		j, err := strconv.Atoi(i)
		if err != nil {
			panic(err)
		}
		resolutionPair = append(resolutionPair, j)
	}

	if err := filepath.Walk(rootDir, visit); err != nil {
		ERRLOG("[walk] %s", err)
	}
	//c := make(chan error)
	//go func() {
	//	c <- filepath.Walk(rootDir, visit)
	//}()
	//
	//if err := <-c; err != nil {
	//	fmt.Println(err)
	//}
}
