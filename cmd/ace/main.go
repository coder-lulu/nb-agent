package main

import (
	"os"
	"runtime/debug"
	_ "time/tzdata"
)

func main() {
	if os.Geteuid() != 0 {
		panic("NewBee Agent must run as root")
	}

	debug.SetGCPercent(10)
	debug.SetMemoryLimit(128 << 20)

	web, err := initWeb()
	if err != nil {
		panic(err)
	}

	if err = web.Run(); err != nil {
		panic(err)
	}
}
