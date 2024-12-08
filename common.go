package main

import (
	"fmt"
	"io"
	"log"
	"os"
	"sync"

	"tinygo.org/x/bluetooth"
)

func Must(err error) {
	if err != nil {
		log.Fatalln(err)
	}
}

func Must1[T any](t T, err error) T {
	if err != nil {
		log.Fatalln(err)
	}
	return t
}

func Min(a, b int) int {
	if a < b {
		return a
	}
	return b
}

// Randomly generated by `uuidgen`.
var (
	uuidService = Must1(bluetooth.ParseUUID(`923BFB18-A711-4923-82A8-988AD38AF7C1`))
	uuidTx      = Must1(bluetooth.ParseUUID(`3F33F755-C5A9-4F25-B0CA-8BFFEF13B905`))
	uuidRx      = Must1(bluetooth.ParseUUID(`41F94EAB-B906-4AE6-BEB9-0D5CA55EC4CB`))
	uuidCtrl    = Must1(bluetooth.ParseUUID(`A004120C-300F-4049-8280-E98AC615ACA1`))
)

func splitWrite(w io.Writer, p []byte) (int, error) {
	// Anybody knows the max packet size of bluetooth?
	// Without a limit, there will be an error.
	//
	// GetMTU()?
	const maxPacketSize = 64

	count := 0
	for len(p) > 0 {
		n, err := w.Write(p[:Min(maxPacketSize, len(p))])
		count += n
		if err != nil {
			return count, err
		}
		p = p[n:]
	}
	return count, nil
}

var (
	errConnClosed = fmt.Errorf(`tcp-over-bt: connection closed`)
)

func Stream(a, b io.ReadWriter) {
	wg := sync.WaitGroup{}
	wg.Add(2)

	go func() {
		defer wg.Done()
		io.Copy(a, b)
	}()

	go func() {
		defer wg.Done()
		io.Copy(b, a)
	}()

	wg.Wait()
}

var Stdio io.ReadWriter = struct {
	io.Reader
	io.Writer
}{
	os.Stdin,
	os.Stdout,
}
