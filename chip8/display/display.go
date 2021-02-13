package display

const (
	DisplayWidth  int = 64
	DisplayHeight int = 32
)

type Drawer interface {
	Draw(pixels [DisplayHeight][DisplayWidth]bool) error
}

type Display struct {
	pixels [DisplayHeight][DisplayWidth]bool
	drawer Drawer
}

func NewDisplay(drawer Drawer) *Display {
	return &Display{
		drawer: drawer,
	}
}

func (d *Display) Clear() error {
	for y := range d.pixels {
		for x := range d.pixels[y] {
			d.pixels[y][x] = false
		}
	}

	return d.drawer.Draw(d.pixels)
}

func (d *Display) DrawSprite(x, y uint8, sprite []uint8) (uint8, error) {
	startX := int(x)
	startY := int(y)

	vf := uint8(0)

	for row := range sprite {
		if startY+row >= DisplayHeight {
			break
		}

		for col := 0; col < 8; col++ {
			if startX+col >= DisplayWidth {
				break
			}

			current := d.pixels[startY+row][startX+col]
			new := (sprite[row]>>(7-col))&1 != 0

			if current && new {
				d.pixels[startY+row][startX+col] = false
				vf = 1
			} else if !current && new {
				d.pixels[startY+row][startX+col] = true
			}
		}
	}

	return vf, d.drawer.Draw(d.pixels)
}
