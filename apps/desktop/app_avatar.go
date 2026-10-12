package main

import (
	"bytes"
	"encoding/base64"
	"errors"
	wruntime "github.com/wailsapp/wails/v2/pkg/runtime"
	"image"
	_ "image/jpeg"
	"image/png"
	"kaelix/internal/store"
	"os"
)

func (a *App) ChooseProfilePhoto() (store.Profile, error) {
	if a.store == nil {
		return store.Profile{}, errors.New("banco local indisponível")
	}
	path, err := wruntime.OpenFileDialog(a.ctx, wruntime.OpenDialogOptions{Title: "Selecionar foto do perfil", Filters: []wruntime.FileFilter{{DisplayName: "Imagens PNG e JPEG", Pattern: "*.png;*.jpg;*.jpeg"}}})
	if err != nil {
		return store.Profile{}, err
	}
	if path == "" {
		return a.GetProfile()
	}
	file, err := os.Open(path)
	if err != nil {
		return store.Profile{}, err
	}
	defer file.Close()
	info, err := file.Stat()
	if err != nil || info.Size() > 10<<20 {
		return store.Profile{}, errors.New("selecione uma imagem de até 10 MB")
	}
	config, _, err := image.DecodeConfig(file)
	if err != nil || config.Width <= 0 || config.Height <= 0 || int64(config.Width)*int64(config.Height) > 40000000 {
		return store.Profile{}, errors.New("imagem inválida ou muito grande")
	}
	file.Seek(0, 0)
	source, _, err := image.Decode(file)
	if err != nil {
		return store.Profile{}, errors.New("não foi possível ler a imagem")
	}
	bounds := source.Bounds()
	side := min(bounds.Dx(), bounds.Dy())
	left := bounds.Min.X + (bounds.Dx()-side)/2
	top := bounds.Min.Y + (bounds.Dy()-side)/2
	thumb := image.NewRGBA(image.Rect(0, 0, 256, 256))
	for y := 0; y < 256; y++ {
		for x := 0; x < 256; x++ {
			thumb.Set(x, y, source.At(left+x*side/256, top+y*side/256))
		}
	}
	var data bytes.Buffer
	if err = png.Encode(&data, thumb); err != nil {
		return store.Profile{}, err
	}
	return a.store.SaveAvatar(a.ctx, "data:image/png;base64,"+base64.StdEncoding.EncodeToString(data.Bytes()))
}
func (a *App) RemoveProfilePhoto() (store.Profile, error) {
	if a.store == nil {
		return store.Profile{}, errors.New("banco local indisponível")
	}
	return a.store.SaveAvatar(a.ctx, "")
}
