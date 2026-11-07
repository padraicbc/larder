package nutrition

import (
	"context"
	"errors"
	"strings"

	"github.com/padraicbc/larder/internal/llm"
)

// NormalizeBarcode keeps only digits and accepts EAN-8, UPC-A (12), EAN-13 and
// GTIN-14 with a valid GS1 check digit. UPC-A is stored as EAN-13 (leading 0)
// so the same pack always has one key. An empty or invalid code returns false.
func NormalizeBarcode(s string) (string, bool) {
	var b strings.Builder
	for _, r := range s {
		switch {
		case r >= '0' && r <= '9':
			b.WriteRune(r)
		case r == ' ' || r == '-':
		default:
			return "", false
		}
	}
	d := b.String()
	switch len(d) {
	case 8, 13, 14:
	case 12:
		d = "0" + d
	default:
		return "", false
	}
	sum := 0
	for i := 0; i < len(d)-1; i++ {
		n := int(d[len(d)-2-i] - '0') // weights 3,1,3,1... from the right of the payload
		if i%2 == 0 {
			n *= 3
		}
		sum += n
	}
	if (10-sum%10)%10 != int(d[len(d)-1]-'0') {
		return "", false
	}
	return d, true
}

// ReadBarcodePhoto reads the digits printed under a barcode in a photo. It is
// the fallback when the browser or phone can't scan live. The digits must pass
// the GS1 check digit, so a misread is rejected instead of guessed.
func ReadBarcodePhoto(ctx context.Context, c llm.VisionClient, images []llm.Image) (string, error) {
	if len(images) != 1 {
		return "", errors.New("add one clear photo of the barcode")
	}
	var out struct {
		Barcode string `json:"barcode"`
	}
	err := llm.CompleteJSONWithImages(ctx, c, "Read product barcodes accurately. Respond with ONLY JSON.", `TASK: barcode_photo
Find the product barcode in this photo and copy the digits printed beneath the bars exactly (EAN-13, EAN-8 or UPC-A, usually 8, 12 or 13 digits). Never guess or complete digits you cannot see.
Return {"barcode":"digits only, or empty if you cannot read all of them"}.`, images, &out)
	if err != nil {
		return "", err
	}
	code, ok := NormalizeBarcode(out.Barcode)
	if !ok {
		return "", errors.New("couldn't read a barcode in that photo; get closer so the digits are sharp, or type them")
	}
	return code, nil
}
