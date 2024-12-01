//go:build darwin

package main

import (
	"bytes"
	"io"
	"log"
	"os"
	"sync"

	"tinygo.org/x/bluetooth"
)

type Host struct {
	adapter *bluetooth.Adapter

	device bluetooth.Device
	svc    bluetooth.DeviceService
	rx, tx bluetooth.DeviceCharacteristic

	mu      sync.Mutex
	txBuf   *bytes.Buffer
	txBufCh chan struct{}
}

func NewHost() *Host {
	h := &Host{
		adapter: bluetooth.DefaultAdapter,
		txBuf:   bytes.NewBuffer(nil),
		txBufCh: make(chan struct{}),
	}

	h.adapter.SetConnectHandler(func(device bluetooth.Device, connected bool) {
		log.Println(`Connect:`, device, connected)
	})

	Must(h.adapter.Enable())

	return h
}

// abe46b44-7f25-a25f-c915-b7b4839d39e7
func (h *Host) Connect(address string) {
	addr := bluetooth.Address{}
	addr.Set(address)
	device, err := h.adapter.Connect(addr, bluetooth.ConnectionParams{})
	if err != nil {
		log.Fatalln(err)
	}
	h.device = device
	services, err := device.DiscoverServices([]bluetooth.UUID{uuidService})
	if err != nil {
		log.Fatalln(err)
	}
	h.svc = services[0]
	chs, err := h.svc.DiscoverCharacteristics([]bluetooth.UUID{uuidTx, uuidRx})
	if err != nil {
		log.Fatalln(err)
	}
	h.tx, h.rx = chs[0], chs[1]
	Must(h.tx.EnableNotifications(func(buf []byte) {
		h.mu.Lock()
		defer h.mu.Unlock()
		h.txBuf.Write(buf)
		select {
		case h.txBufCh <- struct{}{}:
		default:
		}
	}))
}

func (h *Host) Read(p []byte) (int, error) {
	h.mu.Lock()
	if h.txBuf.Len() <= 0 {
		h.mu.Unlock()
		<-h.txBufCh
		h.mu.Lock()
	}
	n, err := h.txBuf.Read(p)
	h.mu.Unlock()
	return n, err
}

func (h *Host) Write(p []byte) (int, error) {
	return splitWrite(h.rx, p)
}

func main() {
	log.SetFlags(log.Flags() | log.Lshortfile)
	h := NewHost()
	h.Connect(`abe46b44-7f25-a25f-c915-b7b4839d39e7`)
	go func() {
		defer log.Println(`exited`)
		Must1(io.Copy(os.Stdout, h))
	}()
	Must1(io.Copy(h, os.Stdin))
}
