package main

import (
	wfn "gitlab.blockfactory.com/sytrax/rfid_ui/embed"
	"runtime"
)

func main() {
	runtime.GOMAXPROCS(runtime.NumCPU())
	wfn.SetupServer()
}
