/*
A simple joystick program.  The goals of this program are to:
1. figure out how SDL handles joysticks
1a. use Go
2. End up with something that helps train my children to use a joystick/gamepad
*/
package main

import (
	"container/list"
	"fmt"
	"github.com/jonhanks/Go-SDL/sdl"
	"github.com/jonhanks/Go-SDL/ttf"
	"math/rand"
	"os"
	"runtime"
	//"runtime/pprof"
	//"strconv"
	"time"
)

const (
	// screen size
	WIDTH  = 1024
	HEIGHT = 768

	// width of the blocks
	RWIDTH  = 20
	RHEIGHT = 20
	STEP    = 15.0
	// step size increase per button press
	BIGMULTIPLIER = 40
	HATMULTIPLIER = 0.4

	// goals/targets
	GOALS_SRC = "ABCDEFGHIJKLMNOPQRSTUVWXYZ"
)

// Drawables know how to draw themselves and provide bounding rectangles for collision detection.
type Drawable interface {
	Rect() *sdl.Rect
	Draw(screen *sdl.Surface)
}

// A Goal object is a Drawable that draws a text string
type Goal struct {
	Text    string       // text to display
	Order   int          // ordering of the goals (the idea is that they be collected in order)
	Surface *sdl.Surface // a surface with the rendered text cached on it
	Hidden  bool         // should this be drawn
	X, Y    int          // location
	W, H    int          // size
}

// Create a new Goal object.  Rendering the given rune with the given font
func NewGoal(f *ttf.Font, ch rune, order int) *Goal {
	g := &Goal{}
	g.Text = string(ch)
	g.Order = order
	g.Surface = ttf.RenderUTF8_Blended(f, g.Text, sdl.Color{255, 255, 255, 0})
	g.W, g.H = int(g.Surface.W), int(g.Surface.H)
	return g
}

// Draw the Goal object on the given surface
func (g Goal) Draw(screen *sdl.Surface) {
	if g.Hidden || g.Surface == nil {
		return
	}
	screen.Blit(g.Rect(), g.Surface, nil)
}

// Get the bounding rectangle for the Goal
func (g Goal) Rect() *sdl.Rect {
	return &sdl.Rect{int16(g.X - (g.W / 2)), int16(g.Y - (g.H / 2)), uint16(g.W), uint16(g.H)}
}

// A Marker is the object tracking the joystick location.
type Marker struct {
	Joystick            *sdl.Joystick // the joystick
	X, Y                int           // position 
	Vax, Vay            float32       // velocity due to the button pad
	Vhx, Vhy            float32       // velocity due to the hat
	Color               uint32
	Big                 int  // how many buttons are pressed
	lastZero, last2Zero bool // I cannot remember what this is used for
}

// Update the markers position
func (m *Marker) Update() {
	if m == nil {
		return
	}
	m.X += int(STEP*m.Vax) + int(STEP*m.Vhx*HATMULTIPLIER)
	m.Y += int(STEP*m.Vay) + int(STEP*m.Vhy*HATMULTIPLIER)
	if m.X < 0 {
		m.X += WIDTH
	}
	if m.X >= WIDTH {
		m.X -= WIDTH
	}
	if m.Y < 0 {
		m.Y += HEIGHT
	}
	if m.Y >= HEIGHT {
		m.Y -= HEIGHT
	}
	m.last2Zero = m.lastZero
	if m.Vax == 0.0 && m.Vay == 0.0 && m.Vhx == 0.0 && m.Vhy == 0.0 {
		m.lastZero = true
	} else {
		m.lastZero = false
		m.last2Zero = false
	}
}

// Close the joystick associated with the marker
func (m *Marker) Close() {
	if m != nil {
		if m.Joystick != nil {
			m.Joystick.Close()
			m.Joystick = nil
		}
	}
}

// Get the bounding rectangle of the marker
func (m Marker) Rect() *sdl.Rect {
	var w, h int = RWIDTH, RHEIGHT
	w += int(BIGMULTIPLIER * m.Big)
	h += int(BIGMULTIPLIER * m.Big)
	return &sdl.Rect{int16(m.X - (w / 2)), int16(m.Y - (h / 2)), uint16(w), uint16(h)}
}

// draw the marker
func (m Marker) Draw(screen *sdl.Surface) {
	screen.FillRect(m.Rect(), m.Color)
}

// Does the marker intersect a given rectangle.
func (m Marker) Intersects(r *sdl.Rect) bool {
	s := m.Rect()
	if int(s.X) > (int(r.X)+int(r.W)) || (int(s.X)+int(s.W)) < int(r.X) {
		return false
	}
	if int(s.Y) > (int(r.Y)+int(r.H)) || (int(s.Y)+int(s.H)) < int(r.Y) {
		return false
	}
	return true
}

// Draw the given list of Drawables on the surface.  Items should be a list of Drawables
func draw(screen *sdl.Surface, items *list.List) {
	screen.FillRect(nil, uint32(0x00202020))
	for cur := items.Front(); cur != nil; cur = cur.Next() {
		if d, ok := cur.Value.(Drawable); ok {
			d.Draw(screen)
		}
	}
}

// timeLoop generates a value on c at periodic intervals
func timeLoop(c chan bool) {
	for {
		time.Sleep( /*time.Millisecond*40*/ time.Second / 30)
		c <- true
	}
}

//The main loop.  Handles drawing, events, ...  This should be broken up into a smaller set of functions
// if more event logic is handled.
func mainLoop(screen *sdl.Surface, markers []Marker, goals []*Goal) {
	var curGoal int

	timer := make(chan bool, 0)

	running := true
	redraw := true
	requestRedraw := false
	stickCount := len(markers)

	// start the timer
	go timeLoop(timer)
	for running {
		if redraw {
			items := list.New()
			nextGoal := false
			var curRect *sdl.Rect
			if curGoal >= 0 && curGoal < len(goals) {
				curRect = goals[curGoal].Rect()
			}
			for i := 0; i < stickCount; i++ {
				markers[i].Update()
				items.PushBack(markers[i])

				if curRect != nil {
					if markers[i].Intersects(curRect) {
						nextGoal = true
					}
				}
			}
			if nextGoal {
				curGoal++
				if curGoal >= len(goals) {
					curGoal = 0
				}
			}
			if curGoal >= 0 && curGoal < len(goals) {
				items.PushBack(goals[curGoal])
			}

			draw(screen, items)
			screen.Flip()
			//fmt.Printf(".")
			redraw = false
			requestRedraw = false
		}
		select {
		case <-timer:
			zeroCnt := 0
			for _, m := range markers {
				if m.last2Zero {
					zeroCnt++
				}
			}
			if zeroCnt < stickCount || requestRedraw {
				redraw = true
			}
		case _event := <-sdl.Events:
			switch e := _event.(type) {
			case sdl.QuitEvent:
				running = false

			case sdl.KeyboardEvent:
				if e.Keysym.Sym == sdl.K_ESCAPE || e.Keysym.Sym == sdl.K_q {
					running = false
				}

			case sdl.JoyAxisEvent:
				if e.Axis < 2 {
					val := float32(0.0)
					if e.Value > 2000 || e.Value < -2000 {
						val = float32(e.Value) / float32(uint32(0x0ffff))
					}
					//fmt.Println("got joystick axis event ", e)

					if e.Axis == 0 {
						markers[e.Which].Vax = val
					} else {
						markers[e.Which].Vay = val
					}
					requestRedraw = true
				}

			case sdl.JoyButtonEvent:
				if e.State > 0 {
					markers[e.Which].Big++
				} else {
					markers[e.Which].Big--
				}
				if markers[e.Which].Big < 0 {
					markers[e.Which].Big = 0
				}
				requestRedraw = true

			case sdl.JoyHatEvent:

				switch e.Value {
				case sdl.HAT_CENTERED:
					markers[e.Which].Vhx = 0.0
					markers[e.Which].Vhy = 0.0
				case sdl.HAT_UP:
					markers[e.Which].Vhx = 0.0
					markers[e.Which].Vhy = -1.0
				case sdl.HAT_RIGHT:
					markers[e.Which].Vhx = 1.0
					markers[e.Which].Vhy = 0.0
				case sdl.HAT_DOWN:
					markers[e.Which].Vhx = 0.0
					markers[e.Which].Vhy = 1.0
				case sdl.HAT_LEFT:
					markers[e.Which].Vhx = -1.0
					markers[e.Which].Vhy = 0.0
				case sdl.HAT_RIGHTUP:
					markers[e.Which].Vhx = 1.0
					markers[e.Which].Vhy = -1.0
				case sdl.HAT_RIGHTDOWN:
					markers[e.Which].Vhx = 1.0
					markers[e.Which].Vhy = 1.0
				case sdl.HAT_LEFTUP:
					markers[e.Which].Vhx = -1.0
					markers[e.Which].Vhy = -1.0
				case sdl.HAT_LEFTDOWN:
					markers[e.Which].Vhx = -1.0
					markers[e.Which].Vhy = 1.0
				}
				//fmt.Println("Hat event ", e, " (",markers[e.Which].Vhx,",",markers[e.Which].Vhy,")")
				requestRedraw = true
			case sdl.ResizeEvent:
				//println("resize screen ", e.W, e.H)
				panic("Resize not supported yet")

				//screen = sdl.SetVideoMode(int(e.W), int(e.H), 32, sdl.RESIZABLE)

				//if screen == nil {
				//	fmt.Println(sdl.GetError())
				//}
			}
		}
		// yeild to allow other activities (such as the timer loop)
		runtime.Gosched()
	}
}

func main() {
	//runtime.GOMAXPROCS(runtime.NumCPU()*2)

	var err error
	os.Setenv("SDL_VIDEODRIVER", "x11")

	rand.Seed(time.Now().Unix())

	GOALS := []rune(GOALS_SRC)

	runtime.GOMAXPROCS(1)
	//f, _ := os.Create("prof.dat")
	//pprof.StartCPUProfile(f)
	//defer pprof.StopCPUProfile()

	if sdl.Init(sdl.INIT_EVERYTHING) != 0 {
		fmt.Println(sdl.GetError())
		return
	}
	defer sdl.Quit()

	// load the font system and a font
	if err = ttf.Init(); err != nil {
		fmt.Println(err)
		return
	}
	defer ttf.Quit()
	var fnt *ttf.Font
	if fnt, err = ttf.OpenFont("font.ttf", 60); err != nil {
		fmt.Println(sdl.GetError())
		return
	}
	defer fnt.Close()

	// build the goals
	goals := make([]*Goal, len(GOALS))
	for i, ch := range GOALS {
		goals[i] = NewGoal(fnt, ch, i)
		goals[i].X = goals[i].W/2 + rand.Intn(WIDTH-goals[i].W)
		goals[i].Y = goals[i].H/2 + rand.Intn(HEIGHT-goals[i].H)
		goals[i].Hidden = false
	}

	stickCount := sdl.NumJoysticks()
	if stickCount == 0 {
		panic("No joysticks available")
	}
	markers := make([]Marker, stickCount)
	fmt.Println("Found ", stickCount, " joysticks:")

	colors := [3]uint32{uint32(0x00aa0000), uint32(0x00009900), uint32(0x00000099)}

	for i := 0; i < stickCount; i++ {
		fmt.Println(i+1, " ", sdl.JoystickName(i))
		markers[i] = Marker{Joystick: sdl.JoystickOpen(i), X: WIDTH / 2, Y: HEIGHT / 2, Color: colors[i%len(colors)]}
		defer markers[i].Close()
	}

	var screen = sdl.SetVideoMode(WIDTH, HEIGHT, 32, 0 /*sdl.RESIZABLE*/)

	if screen == nil {
		fmt.Println(sdl.GetError())
	}

	var video_info = sdl.GetVideoInfo()

	println("HW_available = ", video_info.HW_available)
	println("WM_available = ", video_info.WM_available)
	println("Video_mem = ", video_info.Video_mem, "kb")

	sdl.EnableUNICODE(1)

	sdl.WM_SetCaption("Go-SDL Joystick Test", "")

	if sdl.GetKeyName(270) != "[+]" {
		fmt.Println("GetKeyName broken")
		return
	}
	mainLoop(screen, markers, goals)
}
