package ui

import (
	"bytes"
	_ "embed"
	"image"
	"image/png"
	"log"

	"github.com/danielecanzoneri/lucky-boy/gameboy"
	"github.com/danielecanzoneri/lucky-boy/gameboy/ppu"
	"github.com/hajimehoshi/ebiten/v2"
	"github.com/hajimehoshi/ebiten/v2/ebitenutil"
)

const (
	Scale = 3
)

var (
	// Original palette
	frameImage *ebiten.Image
	// After shader image
	shaderImage *ebiten.Image
)

//go:embed graphics/gbc-shader.kage
var shaderData []byte

var (
	//go:embed graphics/assets/icon16.png
	icon16 []byte
	//go:embed graphics/assets/icon32.png
	icon32 []byte
	//go:embed graphics/assets/icon48.png
	icon48 []byte
	//go:embed graphics/assets/icon64.png
	icon64 []byte
	//go:embed graphics/assets/icon128.png
	icon128 []byte
	//go:embed graphics/assets/icon256.png
	icon256 []byte
)

func (ui *UI) initRenderer(useShader bool) {
	// Set window icon
	decodePNG := func(b []byte) image.Image {
		img, err := png.Decode(bytes.NewReader(b))
		if err != nil {
			panic(err)
		}
		return img
	}
	ebiten.SetWindowIcon([]image.Image{
		decodePNG(icon16),
		decodePNG(icon32),
		decodePNG(icon48),
		decodePNG(icon64),
		decodePNG(icon128),
		decodePNG(icon256),
	})

	// Since game boy is 59.7 FPS but ebiten updates at 60 FPS there are
	// some frames where nothing is drawn. This avoids screen flickering
	ebiten.SetScreenClearedEveryFrame(false)

	// Save when closing
	ebiten.SetWindowClosingHandled(true)

	// Create a single image for the entire frame
	frameImage = ebiten.NewImage(ppu.FrameWidth, ppu.FrameHeight)
	shaderImage = ebiten.NewImage(ppu.FrameWidth, ppu.FrameHeight)

	// Initial window size without the debug panel
	screenWidth, screenHeight := ui.Layout(0, 0)
	ebiten.SetWindowSize(screenWidth, screenHeight)

	if useShader {
		// Load shader
		shader, err := ebiten.NewShader(shaderData)
		if err != nil {
			log.Println("[WARN] could not load shader: ", err)
			ui.Shader = nil
			return
		}

		ui.Shader = shader
		ui.shaderOpts = &ebiten.DrawRectShaderOptions{}
		ui.shaderOpts.Uniforms = map[string]interface{}{
			"LightenScreen": float32(0.0),
		}
	}

	// Reuse pixel buffer to avoid allocations (RGBA = 4 bytes per pixel)
	pixelBufferSize := ppu.FrameWidth * ppu.FrameHeight * 4
	ui.pixelBuffer = make([]byte, pixelBufferSize)
}

// Inherit Ebiten Game interface

func (ui *UI) Update() error {
	// If window is unfocused, stop the game
	//if !ebiten.IsFocused() {
	//	ui.Paused = true
	//} else {
	//	ui.Paused = false
	//}

	// If closing, save game
	if ebiten.IsWindowBeingClosed() {
		ui.Save()
		return ebiten.Termination
	}

	ui.handleInput()

	if ui.debugger.Active {
		ebiten.SetWindowTitle(ui.gameTitle + " (debugging)")
		return ui.debugger.Update()
	} else {
		ebiten.SetWindowTitle(ui.gameTitle)
		// Game updates are called in the audio callback function
		return nil
	}
}

func (ui *UI) applyShader(frame *ebiten.Image) *ebiten.Image {
	if ui.Shader != nil && ui.GameBoy.Model == gameboy.CGB {
		ui.shaderOpts.Images[0] = frame
		shaderImage.DrawRectShader(
			ppu.FrameWidth, ppu.FrameHeight,
			ui.Shader, ui.shaderOpts,
		)
		return shaderImage
	} else {
		return frame
	}
}

func (ui *UI) Draw(screen *ebiten.Image) {
	// Update the frame image with the current frame in the PPU
	frameBuffer, previousBuffer := ui.GameBoy.PPU.GetFrame()

	// Convert frame buffer to RGBA pixels
	for y := range ppu.FrameHeight {
		for x := range ppu.FrameWidth {
			colorId0 := frameBuffer[y][x]
			colorId1 := previousBuffer[y][x]
			c0 := ui.palette.Get(colorId0)
			c1 := ui.palette.Get(colorId1)

			// Direct conversion to RGBA (16 bit)
			r0, g0, b0, a0 := c0.RGBA()
			r1, g1, b1, a1 := c1.RGBA()

			idx := (y*ppu.FrameWidth + x) * 4
			ui.pixelBuffer[idx] = uint8(r0>>9) + uint8(r1>>9)
			ui.pixelBuffer[idx+1] = uint8(g0>>9) + uint8(g1>>9)
			ui.pixelBuffer[idx+2] = uint8(b0>>9) + uint8(b1>>9)
			ui.pixelBuffer[idx+3] = uint8(a0>>9) + uint8(a1>>9)
		}
	}

	// Write all pixels at once
	frameImage.WritePixels(ui.pixelBuffer)

	// Apply shader
	imageToDraw := ui.applyShader(frameImage)

	if ui.debugger.Active {
		ui.debugger.Draw(screen, imageToDraw)
		return
	}

	// Draw the entire frame at once with scaling
	op := &ebiten.DrawImageOptions{}
	op.GeoM.Scale(Scale, Scale)
	screen.DrawImage(imageToDraw, op)

	if ui.debugStringTimer > 0 {
		ebitenutil.DebugPrint(screen, ui.debugString)
		ui.debugStringTimer--
	}
}

func (ui *UI) Layout(_, _ int) (int, int) {
	// Adjust the layout based on whether the debugger is visible
	if ui.debugger.Active {
		return ui.debugger.Layout(0, 0)
	} else {
		return Scale * ppu.FrameWidth, Scale * ppu.FrameHeight
	}
}
