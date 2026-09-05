package main

import "github.com/emersonjoe/trilha"

// A hand-written main: the generator must not emit its own.
func main() {
	a := newApp()
	trilha.Run(a)
}
