//go:build darwin

package main

import (
	"bytes"
	"io"
	"log"
	"os"

	"tinygo.org/x/bluetooth"
)

type Host struct {
	adapter *bluetooth.Adapter

	device bluetooth.Device
	svc    bluetooth.DeviceService
	rx, tx bluetooth.DeviceCharacteristic

	txBuf   *bytes.Buffer
	txBufCh chan []byte
}

func NewHost() *Host {
	h := &Host{
		adapter: bluetooth.DefaultAdapter,
		txBuf:   bytes.NewBuffer(nil),
		txBufCh: make(chan []byte),
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
		h.txBufCh <- buf
	}))
}

func (h *Host) Read(p []byte) (int, error) {
	if h.txBuf.Len() <= 0 {
		next := <-h.txBufCh
		h.txBuf.Write(next)
	}
	n, err := h.txBuf.Read(p)
	// log.Println(`Read:`, p[:n], err)
	return n, err
}

func (h *Host) Write(p []byte) (int, error) {
	// log.Println(`Write:`, len(p))
	count := 0
	for len(p) > 0 {
		n, err := h.rx.Write(p[:Min(64, len(p))])
		if err != nil {
			return 0, err
		}
		p = p[n:]
		count += n
	}
	return count, nil
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
