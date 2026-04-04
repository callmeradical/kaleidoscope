package cmd

type breakpoint struct {
	Name   string
	Width  int
	Height int
}

var defaultBreakpoints = []breakpoint{
	{"mobile", 375, 812},
	{"tablet", 768, 1024},
	{"desktop", 1280, 720},
	{"wide", 1920, 1080},
}
