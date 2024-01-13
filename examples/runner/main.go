package main

import (
	"flag"
	"fmt"
	"io"
	"net/http"
	"os"
	"runtime"
	"sync"
	"unsafe"

	"github.com/pkujhd/goloader"
)

func init() {
	symPtr := make(map[string]uintptr)
	w := sync.WaitGroup{}
	str := make([]string, 0)
	goloader.RegTypes(symPtr, http.ListenAndServe, http.Dir("/"),
		http.Handler(http.FileServer(http.Dir("/"))), http.FileServer, http.HandleFunc,
		&http.Request{}, &http.Server{}, (&http.ServeMux{}).Handle)
	goloader.RegTypes(symPtr, &w, w.Wait)
	goloader.RegTypes(symPtr, fmt.Sprint, str)
	goloader.RegTypes(symPtr, runtime.LockOSThread, runtime.Stack)

}

func main() {
	var goloaderFile = flag.String("f", "", "go loader builder file.")
	var run = flag.String("r", "main.main", "run functionname")

	flag.Parse()

	f, err := os.Open(*goloaderFile)
	if err != nil {
		fmt.Printf("open file:%s failed!\n", *goloaderFile)
		return
	}
	reader := io.Reader(f)
	defer f.Close()
	linker, err := goloader.UnSerialize(reader)
	if err != nil {
		fmt.Printf("unserialize file:%s failed!error:%s\n", *goloaderFile, err)
		return
	}

	symPtr := make(map[string]uintptr)
	err = goloader.RegSymbol(symPtr)
	if err != nil {
		fmt.Printf("goloader RegTypes failed!error:%s\n", err)
		return
	}

	err = runMain(linker, symPtr, *run)
	if err != nil {
		fmt.Printf("run function failed!error:%s\n", err)
		return
	}
}

func runMain(linker *goloader.Linker, symPtr map[string]uintptr, run string) error {
	codeModule, err := goloader.Load(linker, symPtr)
	if err != nil {
		return err
	}

	runFuncPtr := codeModule.Syms[run]
	if runFuncPtr == 0 {
		return err
	}
	funcPtrContainer := (uintptr)(unsafe.Pointer(&runFuncPtr))
	runFunc := *(*func())(unsafe.Pointer(&funcPtrContainer))
	runFunc()
	os.Stdout.Sync()
	codeModule.Unload()
	return nil
}
