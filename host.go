//go:build darwin

package main

import (
	"bytes"
	"fmt"
	"log"
	"os"
	"sync"
	"time"

	"tinygo.org/x/bluetooth"
)

type Host struct {
	adapter *bluetooth.Adapter

	device bluetooth.Device
	svc    bluetooth.DeviceService
	rx, tx bluetooth.DeviceCharacteristic
	ctrl   bluetooth.DeviceCharacteristic

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
		// log.Println(`Connect:`, device, connected)
	})

	Must(h.adapter.Enable())

	return h
}

func (h *Host) Scan(timeout time.Duration) (name string, address string, found bool) {
	now := time.Now()
	uniqueAddresses := map[string]struct{}{}

	if err := h.adapter.Scan(func(a *bluetooth.Adapter, sr bluetooth.ScanResult) {
		if time.Since(now) > timeout {
			h.adapter.StopScan()
		}

		if !sr.HasServiceUUID(uuidService) {
			addr := sr.Address.String()
			if _, ok := uniqueAddresses[addr]; !ok {
				uniqueAddresses[addr] = struct{}{}
				fmt.Fprintln(os.Stderr, `Ignoring`, addr, sr.LocalName())
			}
			return
		}

		name = sr.LocalName()
		address = sr.Address.String()
		found = true

		h.adapter.StopScan()
	}); err != nil {
		log.Println(err)
		return
	}

	return
}

func (h *Host) Connect(address string) {
	addr := bluetooth.Address{}
	addr.Set(address)
	h.device = Must1(h.adapter.Connect(addr, bluetooth.ConnectionParams{}))
	services := Must1(h.device.DiscoverServices([]bluetooth.UUID{uuidService}))
	h.svc = services[0]
	chs := Must1(h.svc.DiscoverCharacteristics([]bluetooth.UUID{uuidTx, uuidRx, uuidCtrl}))
	h.tx, h.rx, h.ctrl = chs[0], chs[1], chs[2]
	Must(h.tx.EnableNotifications(func(buf []byte) {
		h.mu.Lock()
		defer h.mu.Unlock()
		h.txBuf.Write(buf)
		select {
		case h.txBufCh <- struct{}{}:
		default:
		}
	}))

	h.ctrl.Write([]byte(`Greeting from Host`))
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
	fmt.Fprintln(os.Stderr, `Scanning available devices...`)
	name, address, found := h.Scan(time.Minute * 3)
	if !found {
		fmt.Fprintln(os.Stderr, `Device cannot be found`)
		os.Exit(1)
	}
	fmt.Fprintln(os.Stderr, `Connecting to`, address, name)
	h.Connect(address)
	fmt.Fprintln(os.Stderr, `Connected`)

	Stream(h, Stdio)
}
