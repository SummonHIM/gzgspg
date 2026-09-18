package ui

import (
	_ "embed"

	"fyne.io/fyne/v2"
)

//go:embed icon.png
var iconPNG []byte

// appIcon 返回打包进二进制的应用图标。
func appIcon() fyne.Resource {
	return fyne.NewStaticResource("icon.png", iconPNG)
}
