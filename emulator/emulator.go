package emulator

// typedef unsigned char Uint8;
// void AudioCallback(void *userdata, Uint8 *stream, int len);
import "C"

import (
	"chip8/chip8"
	"fmt"
	"math"
	"path/filepath"
	"reflect"
	"time"
	"unsafe"

	sdl "github.com/veandco/go-sdl2/sdl"
)

var keyMap = map[sdl.Scancode]int{
	sdl.SCANCODE_1: 0x1,
	sdl.SCANCODE_2: 0x2,
	sdl.SCANCODE_3: 0x3,
	sdl.SCANCODE_4: 0xC,
	sdl.SCANCODE_Q: 0x4,
	sdl.SCANCODE_W: 0x5,
	sdl.SCANCODE_E: 0x6,
	sdl.SCANCODE_R: 0xD,
	sdl.SCANCODE_A: 0x7,
	sdl.SCANCODE_S: 0x8,
	sdl.SCANCODE_D: 0x9,
	sdl.SCANCODE_F: 0xE,
	sdl.SCANCODE_Z: 0xA,
	sdl.SCANCODE_X: 0x0,
	sdl.SCANCODE_C: 0xB,
	sdl.SCANCODE_V: 0xF,
}

const (
	toneHz   = 440
	sampleHz = 22050
	dPhase   = 2 * math.Pi * toneHz / sampleHz
)

//export AudioCallback
func AudioCallback(userdata unsafe.Pointer, stream *C.Uint8, length C.int) {
	n := int(length)
	hdr := reflect.SliceHeader{Data: uintptr(unsafe.Pointer(stream)), Len: n, Cap: n}
	buf := *(*[]C.Uint8)(unsafe.Pointer(&hdr))

	var phase float64
	for i := 0; i < n; i += 2 {
		phase += dPhase
		sample := C.Uint8((math.Sin(phase) + 0.999999) * 128)
		buf[i] = sample
		buf[i+1] = sample
	}
}

type beeper struct {
	deviceId sdl.AudioDeviceID
}

func newBeeper() (*beeper, error) {
	spec := sdl.AudioSpec{
		Freq:     sampleHz,
		Format:   sdl.AUDIO_U8,
		Channels: 2,
		Samples:  1024,
		Callback: sdl.AudioCallback(C.AudioCallback),
	}

	if err := sdl.OpenAudio(&spec, nil); err != nil {
		return nil, err
	}

	return &beeper{}, nil
}

func (b *beeper) destroy() {
	sdl.CloseAudio()
}

func (b *beeper) Beep() {
	sdl.PauseAudio(false)

	go func() {
		timer := time.NewTimer(time.Second / 10)
		select {
		case <-timer.C:
			sdl.PauseAudio(true)
		}
	}()
}

type keys struct {
	current  [16]bool
	previous [16]bool
}

func (k *keys) startFrame() {
	copy(k.previous[:], k.current[:])
}

func (k *keys) handleEvent(e *sdl.KeyboardEvent) {
	switch e.Type {
	case sdl.KEYUP:
		if key, ok := keyMap[e.Keysym.Scancode]; ok {
			k.current[key] = false
		}
	case sdl.KEYDOWN:
		if key, ok := keyMap[e.Keysym.Scancode]; ok {
			k.current[key] = true
		}
	}
}

func (k *keys) IsKeyDown(i uint8) bool {
	return k.current[i]
}

func (k *keys) WasKeyReleased(i uint8) bool {
	return !k.current[i] && k.previous[i]
}

type window struct {
	window     *sdl.Window
	renderer   *sdl.Renderer
	backbuffer *sdl.Texture
}

func newWindow(filename string) (*window, error) {
	w, err := sdl.CreateWindow(fmt.Sprintf("Chip 8 - %s", filepath.Base(filename)), sdl.WINDOWPOS_UNDEFINED, sdl.WINDOWPOS_UNDEFINED, 640, 320, sdl.WINDOW_SHOWN)
	if err != nil {
		return nil, err
	}

	renderer, err := sdl.CreateRenderer(w, -1, 0)
	if err != nil {
		w.Destroy()
		return nil, err
	}

	backbuffer, err := renderer.CreateTexture(sdl.PIXELFORMAT_ABGR8888, sdl.TEXTUREACCESS_TARGET, int32(chip8.DISPLAY_WIDTH), int32(chip8.DISPLAY_HEIGHT))
	if err != nil {
		renderer.Destroy()
		w.Destroy()
		return nil, err
	}

	return &window{
		window:     w,
		renderer:   renderer,
		backbuffer: backbuffer,
	}, nil
}

func (d *window) destroy() {
	d.backbuffer.Destroy()
	d.renderer.Destroy()
	d.window.Destroy()
}

func (d *window) present() {
	d.renderer.SetDrawColor(255, 0, 0, 255)
	d.renderer.Clear()

	d.renderer.Copy(d.backbuffer, nil, nil)

	d.renderer.Present()
}

func (d *window) Draw(pixels [chip8.DISPLAY_HEIGHT][chip8.DISPLAY_WIDTH]bool) {
	target := d.renderer.GetRenderTarget()
	d.renderer.SetRenderTarget(d.backbuffer)

	for y := range pixels {
		for x := range pixels[y] {
			if pixels[y][x] {
				d.renderer.SetDrawColor(0, 0, 0, 255)
			} else {
				d.renderer.SetDrawColor(255, 255, 255, 255)
			}

			d.renderer.DrawPoint(int32(x), int32(y))
		}
	}

	d.renderer.SetRenderTarget(target)
}

func Run(filename string) error {
	if err := sdl.Init(sdl.INIT_EVERYTHING); err != nil {
		return err
	}
	defer sdl.Quit()

	keys := &keys{}

	beeper, err := newBeeper() //&beeper{}
	if err != nil {
		return err
	}
	defer beeper.destroy()

	window, err := newWindow(filename)
	if err != nil {
		return err
	}
	defer window.destroy()

	chip8, err := chip8.New(keys, beeper, window)
	if err != nil {
		return err
	}

	err = chip8.LoadROM(filename)
	if err != nil {
		return err
	}

	currentTime := time.Now()
	accumulator := time.Duration(0)

	dt := time.Duration(16666667)

	for {
		now := time.Now()

		frameTime := now.Sub(currentTime)
		currentTime = now

		accumulator += frameTime

		for accumulator > dt {
			keys.startFrame()

			for event := sdl.PollEvent(); event != nil; event = sdl.PollEvent() {
				switch e := event.(type) {
				case *sdl.QuitEvent:
					return nil
				case *sdl.KeyboardEvent:
					keys.handleEvent(e)
				}

			}

			err = chip8.Cycle()
			if err != nil {
				return err
			}

			accumulator -= dt
		}

		window.present()
	}
}
