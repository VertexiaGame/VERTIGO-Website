package renderer

import (
	"image"
)

func downsample(img *image.NRGBA, scaleFactor int) *image.NRGBA {
	if scaleFactor <= 1 {
		return img
	}
	w := img.Bounds().Dx()
	h := img.Bounds().Dy()
	ow := w / scaleFactor
	oh := h / scaleFactor
	out := image.NewNRGBA(image.Rect(0, 0, ow, oh))
	src := img.Pix
	dst := out.Pix
	sw := img.Stride
	n := uint32(scaleFactor * scaleFactor)
	for y := 0; y < oh; y++ {
		for x := 0; x < ow; x++ {
			var rs, gs, bs, as uint32
			base := y*scaleFactor*sw + x*scaleFactor*4
			for dy := 0; dy < scaleFactor; dy++ {
				o := base + dy*sw
				for dx := 0; dx < scaleFactor; dx++ {
					a := uint32(src[o+3])
					if a != 0 {
						rs += uint32(src[o]) * a
						gs += uint32(src[o+1]) * a
						bs += uint32(src[o+2]) * a
					}
					as += a
					o += 4
				}
			}
			d := (y*out.Stride + x*4)
			if as == 0 {
				dst[d] = 0
				dst[d+1] = 0
				dst[d+2] = 0
				dst[d+3] = 0
			} else {
				dst[d] = uint8((rs + as/2) / as)
				dst[d+1] = uint8((gs + as/2) / as)
				dst[d+2] = uint8((bs + as/2) / as)
				dst[d+3] = uint8(as / n)
			}
		}
	}
	return out
}