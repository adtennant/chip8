package emulator

// typedef unsigned char Uint8;
// void AudioCallback(void *userdata, Uint8 *stream, int len);
import "C"

import (
	"chip8/chip8"
	"chip8/chip8/display"
	"fmt"
	"math"
	"os"
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

	cyclesPerSecond = 500
)

//export AudioCallback
func AudioCallback(userdata unsafe.Pointer, stream *C.Uint8, length C.int) {
	n := int(length)

	var buf []C.Uint8
	hdr := (*reflect.SliceHeader)(unsafe.Pointer(&buf))
	hdr.Cap = n
	hdr.Len = n
	hdr.Data = uintptr(unsafe.Pointer(stream))

	var phase float64
	for i := 0; i < n; i += 2 {
		phase += dPhase
		sample := C.Uint8((math.Sin(phase) + 0.999999) * 128)
		buf[i] = sample
		buf[i+1] = sample
	}
}

type beeper struct{}

func newBeeper() (*beeper, error) {
	spec := sdl.AudioSpec{
		Freq:     sampleHz,
		Format:   sdl.AUDIO_U8,
		Channels: 2,
		Samples:  1024,
		Callback: sdl.AudioCallback(C.AudioCallback),
	}

	if err := sdl.OpenAudio(&spec, nil); err != nil {
		return nil, fmt.Errorf("failed to open audio: %v", err)
	}

	return &beeper{}, nil
}

func (b *beeper) destroy() {
	sdl.CloseAudio()
}

func (b *beeper) Beep() {
	sdl.PauseAudio(false)

	go func() {
		for {
			<-time.After(time.Second / 5)
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
		return nil, fmt.Errorf("failed to create window: %v", err)
	}

	renderer, err := sdl.CreateRenderer(w, -1, 0)
	if err != nil {
		_ = w.Destroy()
		return nil, fmt.Errorf("failed to create renderer: %v", err)
	}

	backbuffer, err := renderer.CreateTexture(sdl.PIXELFORMAT_ABGR8888, sdl.TEXTUREACCESS_TARGET, int32(display.DisplayWidth), int32(display.DisplayHeight))
	if err != nil {
		_ = renderer.Destroy()
		_ = w.Destroy()
		return nil, fmt.Errorf("failed to create backbuffer: %v", err)
	}

	return &window{
		window:     w,
		renderer:   renderer,
		backbuffer: backbuffer,
	}, nil
}

func (d *window) destroy() {
	_ = d.backbuffer.Destroy()
	_ = d.renderer.Destroy()
	_ = d.window.Destroy()
}

func (d *window) present() error {
	if err := d.renderer.SetDrawColor(255, 0, 0, 255); err != nil {
		return fmt.Errorf("failed to set draw color: %v", err)
	}

	if err := d.renderer.Clear(); err != nil {
		return fmt.Errorf("failed to clear: %v", err)
	}

	if err := d.renderer.Copy(d.backbuffer, nil, nil); err != nil {
		return fmt.Errorf("failed to copy backbuffer: %v", err)
	}

	d.renderer.Present()

	return nil
}

func (d *window) Draw(pixels [display.DisplayHeight][display.DisplayWidth]bool) error {
	target := d.renderer.GetRenderTarget()

	if err := d.renderer.SetRenderTarget(d.backbuffer); err != nil {
		return fmt.Errorf("failed to set render target: %v", err)
	}

	for y := range pixels {
		for x := range pixels[y] {
			if pixels[y][x] {
				if err := d.renderer.SetDrawColor(0, 0, 0, 255); err != nil {
					return fmt.Errorf("failed to set draw color: %v", err)
				}
			} else {
				if err := d.renderer.SetDrawColor(255, 255, 255, 255); err != nil {
					return fmt.Errorf("failed to set draw color: %v", err)
				}
			}

			if err := d.renderer.DrawPoint(int32(x), int32(y)); err != nil {
				return fmt.Errorf("failed to draw point: %v", err)
			}
		}
	}

	if err := d.renderer.SetRenderTarget(target); err != nil {
		return fmt.Errorf("failed to restore render target: %v", err)
	}

	return nil
}

func Run(filename string) error {
	if err := sdl.Init(sdl.INIT_EVERYTHING); err != nil {
		return fmt.Errorf("failed to init SDL: %v", err)
	}
	defer sdl.Quit()

	keys := &keys{}

	beeper, err := newBeeper()
	if err != nil {
		return fmt.Errorf("failed to init beeper: %v", err)
	}
	defer beeper.destroy()

	window, err := newWindow(filename)
	if err != nil {
		return fmt.Errorf("failed to init window: %v", err)
	}
	defer window.destroy()

	chip8, err := chip8.New(keys, beeper, window)
	if err != nil {
		return fmt.Errorf("failed to init chip8: %v", err)
	}

	rom, err := os.Open(filename)
	if err != nil {
		return fmt.Errorf("failed to open ROM file: %v", err)
	}

	defer rom.Close()

	err = chip8.LoadROM(rom)
	if err != nil {
		return fmt.Errorf("failed to load ROM file: %v", err)
	}

	currentTime := time.Now()
	accumulator := time.Duration(0)

	dt := time.Duration(time.Second.Nanoseconds() / cyclesPerSecond)

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
				return fmt.Errorf("failed to cycle: %v", err)
			}

			accumulator -= dt
		}

		err = window.present()
		if err != nil {
			return fmt.Errorf("failed to present: %v", err)
		}
	}
}
