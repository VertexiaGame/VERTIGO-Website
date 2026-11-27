package handlers

import (
	"bytes"
	"errors"
	"image"
	_ "image/gif"
	_ "image/jpeg"
	"image/png"
	"io"
	"mime/multipart"
	"net/http"
	"regexp"
	"slices"
	"strconv"

	_ "golang.org/x/image/webp"
	"vertexia-frontend/backend/config"
)

const (
	defaultUploadMaxBytes = 40 << 20
	// maxImageDimension caps either side of any decoded upload so a small
	// compressed file cannot expand into gigabytes of pixels.
	maxImageDimension = 8192
	// maxImagePixels caps the decoded area (32 megapixels, about 128 MiB as
	// RGBA) for the same reason.
	maxImagePixels = 8192 * 4096
)

var (
	errUploadTooLarge = errors.New("file is too large")
	errUploadEmpty    = errors.New("empty file")
	errInvalidImage   = errors.New("invalid image file")
	errImageTooLarge  = errors.New("image dimensions are too large (max 8192x8192)")
)

// Image formats accepted by sniffImage, named as image.DecodeConfig reports
// them.
const (
	imgPNG  = "png"
	imgJPEG = "jpeg"
	imgGIF  = "gif"
	imgWEBP = "webp"
)

var imageFormatExt = map[string]string{imgPNG: ".png", imgJPEG: ".jpg", imgGIF: ".gif", imgWEBP: ".webp"}

var sniffedImageFormat = map[string]string{
	"image/png":  imgPNG,
	"image/jpeg": imgJPEG,
	"image/gif":  imgGIF,
	"image/webp": imgWEBP,
}

// uploadMaxBytes is the global UPLOAD_MAX_BYTES cap.
func uploadMaxBytes() int64 {
	if config.Global != nil && config.Global.UploadMaxBytes > 0 {
		return config.Global.UploadMaxBytes
	}
	return defaultUploadMaxBytes
}

// uploadLimit returns the smaller of a per-type limit and UPLOAD_MAX_BYTES.
func uploadLimit(perType int64) int64 {
	limit := uploadMaxBytes()
	if perType > 0 && perType < limit {
		return perType
	}
	return limit
}

// readLimited reads r and fails when it holds more than limit bytes. The
// size is measured, never taken from headers.
func readLimited(r io.Reader, limit int64) ([]byte, error) {
	data, err := io.ReadAll(io.LimitReader(r, limit+1))
	if err != nil {
		return nil, err
	}
	if int64(len(data)) > limit {
		return nil, errUploadTooLarge
	}
	if len(data) == 0 {
		return nil, errUploadEmpty
	}
	return data, nil
}

// readUpload reads a multipart file with a hard size limit.
func readUpload(header *multipart.FileHeader, limit int64) ([]byte, error) {
	if header == nil {
		return nil, errUploadEmpty
	}
	limit = uploadLimit(limit)
	if header.Size > limit {
		return nil, errUploadTooLarge
	}
	src, err := header.Open()
	if err != nil {
		return nil, err
	}
	defer src.Close()
	return readLimited(src, limit)
}

// sniffImage checks that data is an image in one of the allowed formats
// using both content sniffing and the decoder's header parser, and that its
// dimensions are within bounds. It does not decode pixels.
func sniffImage(data []byte, allowed ...string) (string, image.Config, error) {
	sniffed, ok := sniffedImageFormat[http.DetectContentType(data)]
	if !ok || !slices.Contains(allowed, sniffed) {
		return "", image.Config{}, errInvalidImage
	}
	cfg, format, err := image.DecodeConfig(bytes.NewReader(data))
	if err != nil || format != sniffed {
		return "", image.Config{}, errInvalidImage
	}
	if cfg.Width <= 0 || cfg.Height <= 0 {
		return "", image.Config{}, errInvalidImage
	}
	if cfg.Width > maxImageDimension || cfg.Height > maxImageDimension || cfg.Width*cfg.Height > maxImagePixels {
		return "", image.Config{}, errImageTooLarge
	}
	return format, cfg, nil
}

// decodeImage validates with sniffImage and then decodes the pixels.
func decodeImage(data []byte, allowed ...string) (image.Image, string, error) {
	format, _, err := sniffImage(data, allowed...)
	if err != nil {
		return nil, "", err
	}
	img, decoded, err := image.Decode(bytes.NewReader(data))
	if err != nil || decoded != format {
		return nil, "", errInvalidImage
	}
	return img, format, nil
}

// reencodePNG decodes an untrusted PNG or JPEG and re-encodes it as PNG,
// dropping metadata and anything appended to the file.
func reencodePNG(data []byte) ([]byte, error) {
	img, _, err := decodeImage(data, imgPNG, imgJPEG)
	if err != nil {
		return nil, err
	}
	var buf bytes.Buffer
	if err := png.Encode(&buf, img); err != nil {
		return nil, errors.New("could not save upload")
	}
	return buf.Bytes(), nil
}

// shopItemTypes is the allowlist of shop item directories (the normalized
// categories from models.NormalizeShopCategory).
var shopItemTypes = map[string]bool{"hat": true, "faces": true, "shirts": true, "tshirts": true, "pants": true, "gear": true}

var digitsOnly = regexp.MustCompile(`^[0-9]{1,9}$`)

// parseStrictID parses a positive decimal id with no sign, spaces or other
// characters, optionally followed by one of the allowed suffixes (such as
// ".png").
func parseStrictID(raw string, suffixes ...string) (int, bool) {
	for _, s := range suffixes {
		if len(raw) > len(s) && raw[len(raw)-len(s):] == s {
			raw = raw[:len(raw)-len(s)]
			break
		}
	}
	if !digitsOnly.MatchString(raw) {
		return 0, false
	}
	id, err := strconv.Atoi(raw)
	if err != nil || id <= 0 {
		return 0, false
	}
	return id, true
}
