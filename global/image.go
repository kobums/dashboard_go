package global

import (
	"image"
	"image/color"
	"os"

	"dashboard/global/log"

	"github.com/disintegration/imaging"
)

func MakeThumbnail(w int, h int, filename string, targetFilename string) {
	imgfile, err := os.Open(filename)

	if err != nil {
		return
	}

	defer imgfile.Close()

	imgCfg, _, err := image.DecodeConfig(imgfile)

	if err != nil {
		return
	}

	width := imgCfg.Width
	height := imgCfg.Height

	rate := float64(w) / float64(h)
	target := float64(width) / float64(height)

	newWidth := 0
	newHeight := 0

	x := 0
	y := 0

	if rate > target {
		newWidth = int(float64(h) * target)
		newHeight = h

		x = (w - newWidth) / 2
	} else if rate < target {
		newWidth = w
		newHeight = int(float64(w) / target)

		y = (h - newHeight) / 2
	} else {
		newWidth = w
		newHeight = h
	}

	src, err := imaging.Open(filename)
	if err != nil {
		log.Error().Msg(err.Error())
		return
	}

	src = imaging.Resize(src, newWidth, newHeight, imaging.Lanczos)

	dst := imaging.New(w, h, color.NRGBA{0, 0, 0, 0})
	dst = imaging.Paste(dst, src, image.Pt(x, y))

	err = imaging.Save(dst, targetFilename)
	if err != nil {
		log.Error().Msg(err.Error())
	}
}
