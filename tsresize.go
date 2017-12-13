package main

import (
	"os"
	"fmt"
	"flag"
	"path"
	"path/filepath"
	"strings"
	"strconv"
	"github.com/anthonynsimon/bild/imgio"
	"github.com/anthonynsimon/bild/transform"
	"io"
	"image"
	"golang.org/x/image/tiff"
)

var (
	rootDir          string
	outputDir        string
	targetResolution string
	targetExtension  string
	resolutionPair   []int
	imageEncoder     imgio.Encoder
)

func Printfln(format string, a ...interface{}) (n int, err error) {
	return fmt.Fprintf(os.Stderr, format+"\n", a...)
}

// Set the affine array to scale the image by sx,sy
//func setScaleAffine(sx, sy float64) []float64 {
//	s := make([]float64, 6)
//	s[0] = sx
//	s[1] = 0
//	s[2] = 0
//	s[3] = sy
//	s[4] = 0
//	s[5] = 0
//	return s
//}

// TIFFEncoder returns an encoder to the Tagged Image Format
func TIFFEncoder(compressionType tiff.CompressionType) imgio.Encoder {
	return func(w io.Writer, img image.Image) error {
		return tiff.Encode(w, img, &tiff.Options{Compression: compressionType})
	}
}

func convertImage(sourcePath, destPath string) error {
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
	newBase := fmt.Sprintf("%s.%s", noExtension , targetExtension)
	newPath := path.Join(outputDir, newBase)

	// convert the image
	if err := convertImage(filePath, newPath); err != nil {
		Printfln("[convert] %s", err)
		return nil
	}
	// print the new path.
	fmt.Println(newPath)
	return nil
}

var usage = func() {
	fmt.Fprintf(os.Stderr, "usage of %s:\n", os.Args[0])
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
	fmt.Fprintf(os.Stderr, "\t-type: set the output image type (default=jpeg)\n")
	fmt.Fprintf(os.Stderr, "\t\tavailable image types:\n")
	fmt.Fprintf(os.Stderr, "\t\tjpeg, png\n")
	fmt.Fprintf(os.Stderr, "\t\ttiff: tiff with Deflate compression (alias for tiff-deflate)\n")
	fmt.Fprintf(os.Stderr, "\t\ttiff-lzw: tiff with LZW compression\n")
	fmt.Fprintf(os.Stderr, "\t\ttiff-none: tiff with no compression\n")
	fmt.Fprintf(os.Stderr, "\t-output: set the <destination> directory (default=%s)\n", pwd)
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
		Printfln("[path] no <source> specified")
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
			Printfln("[path] <source> %s does not exist.", rootDir)
			os.Exit(1)
		}
	}

	if outputDir == "" {
		outputDir = path.Join(rootDir, targetResolution)
		Printfln("[path] no <destination>, creating %s", outputDir)
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
		Printfln("[walk] %s", err)
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
