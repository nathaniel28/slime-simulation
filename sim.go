//see https://www.evl.uic.edu/arao/cs594/sdlglsl.html?

package main

import (
	//"fmt"
	"flag"
	"image/color"
	"log"
	"math"
	"math/rand"
	"os"
	"runtime/pprof"
	"sync"
	
	"github.com/veandco/go-sdl2/sdl"
)

const (
	goroutineLimit int = 8
)

/*
func dispatchGoroutines(count, from, to int, fn func()) {
	//TODO
}
*/

type Agent struct {
	x, y, angle float64
}

type TrailMap [][]float64

var agents []Agent

func (t *TrailMap) Init(x, y int) {
	*t = make([][]float64, x)
	for i := 0; i < x; i++ {
		(*t)[i] = make([]float64, y)
	}
}

func (t *TrailMap) sizeX() int { return len(*t) }

func (t *TrailMap) sizeY() int { return len((*t)[0]) }

func (t *TrailMap) inBounds(x, y int) bool { return x >= 0 && x < t.sizeX() && y >= 0 && y < t.sizeY() }

func (t *TrailMap) sampleAround(x, y, size int) float64 { //includes itself
	var sum float64
	for dy := y - size; dy <= y+size; dy++ {
		for dx := x - size; dx <= x+size; dx++ {
			if t.inBounds(dx, dy) {
				sum += (*t)[dx][dy]
			}
		}
	}
	return sum
}

func UpdateMap(curr, next *TrailMap) {
	const evaporationSpeed float64 = 0.5
	var wg sync.WaitGroup
	wg.Add(goroutineLimit)
	startWork := func(fromX, fromY, toX, toY int) {
		defer wg.Done()
		for y := fromY; y < toY; y++ {
			for x := fromX; x < toX; x++ {
				a, b := curr.sampleAround(x, y, 1)/9, (*curr)[x][y]
				if a > b {
					a, b = b, a
				}
				(*next)[x][y] = a+0.92*(b-a)
				evaporatedVal := (*next)[x][y]-evaporationSpeed
				if evaporatedVal > 0 {
					(*next)[x][y] = evaporatedVal
				}
			}
		}
	}
	dx, xs := curr.sizeX()/goroutineLimit, 0
	for i := 0; i < goroutineLimit - 1; i++ {
		go startWork(xs, 0, xs+dx, curr.sizeY())
		xs += dx
	}
	go startWork(xs, 0, curr.sizeX(), curr.sizeY())
	wg.Wait()
}

func AddAgent(x, y, angle float64) {
	agents = append(agents, Agent{x, y, angle})
}

func round(n float64) int { return int(math.Round(n)) }

func (a *Agent) updateGiven(t *TrailMap) {
	const (
		speed        float64 = 0.65
		SO           float64 = 15.0 //sensor offset distance
		SA           float64 = math.Pi / 2 //sensor angle
		SW           int     = 1 //sensor width
		turnStrength float64 = 0.3
	)
	
	a.x += math.Cos(a.angle)*speed
	a.y += math.Sin(a.angle)*speed

	rx, ry := round(a.x), round(a.y)
	
	
	if rx > t.sizeX()-1 {
		a.x = 0
	} else if rx < 0 {
		a.x = float64(t.sizeX() - 1)
	}
	if ry > t.sizeY()-1 {
		a.y = 0
	} else if ry < 0 {
		a.y = float64(t.sizeY() - 1)
	}

	FL := t.sampleAround(round(a.x+SO*math.Cos(a.angle-SA)), round(a.y+SO*math.Sin(a.angle-SA)), SW)
	F := t.sampleAround(round(a.x+SO*math.Cos(a.angle)), round(a.y+SO*math.Sin(a.angle)), SW)
	FR := t.sampleAround(round(a.x+SO*math.Cos(a.angle-SA)), round(a.y+SO*math.Sin(a.angle-SA)), SW)

	switch {
	case F > FL && F > FR:
		break
	case F < FL && F < FR:
		a.angle += turnStrength * 2*rand.Float64()-1
	case FL < FR:
		a.angle += turnStrength * rand.Float64()
	case FR < FL:
		a.angle -= turnStrength * rand.Float64()
	}
}

func UpdateAgents(t *TrailMap) {
	var wg sync.WaitGroup
	wg.Add(goroutineLimit)
	startWork := func(from, to int) {
		defer wg.Done()
		for j := from; j < to; j++ {
			agents[j].updateGiven(t)
		}
	}
	delta, start := len(agents)/goroutineLimit, 0
	for i := 0; i < goroutineLimit - 1; i++ {
		go startWork(start, start+delta)
		start += delta
	}
	go startWork(start, len(agents)-1)
	wg.Wait()
	for i, _ := range agents {
		(*t)[round(agents[i].x)][round(agents[i].y)] = 255
	}
}

func (t *TrailMap) Draw(s *sdl.Surface) {
	s.Lock()
	for y := 0; y < t.sizeY(); y++ {
		for x := 0; x < t.sizeX(); x++ {
			s.Set(x, y, color.Gray{uint8((*t)[x][y])})
		}
	}
	s.Unlock()
}

var cpuprofile = flag.String("cpuprofile", "", "write cpu profile to file")

func main() {
	flag.Parse()
	if *cpuprofile != "" {
		f, err := os.Create(*cpuprofile)
		if err != nil {
			log.Fatal(err)
		}
		pprof.StartCPUProfile(f)
		defer pprof.StopCPUProfile()
	}

	const (
		sizeX = 500
		sizeY = 500
	)
	
	if err := sdl.Init(sdl.INIT_EVERYTHING); err != nil {
		panic(err)
	}
	defer sdl.Quit()

	window, err := sdl.CreateWindow("sim", sdl.WINDOWPOS_UNDEFINED, sdl.WINDOWPOS_UNDEFINED, sizeX, sizeY, sdl.WINDOW_SHOWN)
	if err != nil {
		panic(err)
	}
	defer window.Destroy()

	surface, err := window.GetSurface()
	if err != nil {
		panic(err)
	}

	t := new(TrailMap)
	n := new(TrailMap)
	t.Init(sizeX, sizeY)
	n.Init(sizeX, sizeY)
	
	for i := 0; i < 1000; i++ {
		AddAgent(rand.Float64()*sizeX, rand.Float64()*sizeY, rand.Float64()*math.Pi*2)
	}

	running := true
	for running {
		for event := sdl.PollEvent(); event != nil; event = sdl.PollEvent() {
			switch event.(type) {
			case *sdl.QuitEvent:
				running = false
				break
			}
		}
		UpdateAgents(t)
		UpdateMap(t, n)
		window.UpdateSurface()
		t.Draw(surface)
		t, n = n, t
	}
}
