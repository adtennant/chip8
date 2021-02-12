package main

import (
	"chip8/emulator"
	"flag"
	"fmt"
	"os"
)

func main() {
	flag.Usage = func() {
		fmt.Printf("Usage: %s [FILENAME]\n", os.Args[0])
		flag.PrintDefaults()
	}
	flag.Parse()

	filename := flag.Arg(0)
	if filename == "" {
		flag.Usage()
		os.Exit(1)
	}

	if err := emulator.Run(filename); err != nil {
		panic(err)
	}
}
