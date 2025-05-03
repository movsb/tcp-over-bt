//go:build darwin

package main

import (
	"bytes"
	"fmt"
	"io"
	"log"
	"os"
	"sync"
	"time"

	"github.com/spf13/pflag"
	"tinygo.org/x/bluetooth"
)

type Host struct {
	adapter *bluetooth.Adapter
}

func NewHost() *Host {
	h := &Host{
		adapter: bluetooth.DefaultAdapter,
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

func (h *Host) Connect(address string) Conn {
	addr := bluetooth.Address{}
	addr.Set(address)

	device := Must1(h.adapter.Connect(addr, bluetooth.ConnectionParams{}))
	services := Must1(device.DiscoverServices([]bluetooth.UUID{uuidService}))
	service := services[0]
	chs := Must1(service.DiscoverCharacteristics([]bluetooth.UUID{uuidTx, uuidRx, uuidCtrl}))
	tx, rx, ctrl := chs[0], chs[1], chs[2]

	conn := &HostConn{
		w: NewSegmentedWriter(rx, maxPacketSize),

		txBuf:   bytes.NewBuffer(nil),
		txBufCh: make(chan struct{}),
	}

	// Notification callback is called in a separate goroutine, so it may be in wrong order.
	// https://github.com/tinygo-org/bluetooth/blob/b82048cd9da0fdabb6f2508f461c364184087b3a/gap_darwin.go#L223
	Must(tx.EnableNotifications(func(buf []byte) {
		conn.mu.Lock()
		defer conn.mu.Unlock()
		conn.txBuf.Write(buf)
		select {
		case conn.txBufCh <- struct{}{}:
		default:
		}
	}))

	Must1(ctrl.Write([]byte(`Greeting from Host`)))

	return conn
}

type HostConn struct {
	w io.Writer

	mu      sync.RWMutex
	txBuf   *bytes.Buffer
	txBufCh chan struct{}
}

func (c *HostConn) Read(p []byte) (int, error) {
	c.mu.RLock()
	if c.txBuf.Len() <= 0 {
		c.mu.RUnlock()
		<-c.txBufCh
		c.mu.RLock()
	}
	n, err := c.txBuf.Read(p)
	c.mu.RUnlock()
	if err == io.EOF {
		err = nil
	}
	return n, err
}

func (c *HostConn) Write(p []byte) (int, error) {
	return c.w.Write(p)
}

func main() {
	log.SetFlags(log.Flags() | log.Lshortfile)

	var (
		name    string
		address string
	)

	pflag.StringVarP(&address, `address`, `a`, ``, `Address of the device`)
	pflag.Parse()

	h := NewHost()

	if !pflag.CommandLine.Changed(`address`) {
		fmt.Fprintln(os.Stderr, `Scanning available devices...`)
		var found bool
		name, address, found = h.Scan(time.Minute * 3)
		if !found {
			fmt.Fprintln(os.Stderr, `Device cannot be found`)
			os.Exit(1)
		}
	} else {
		address = Must1(pflag.CommandLine.GetString(`address`))
	}

	fmt.Fprintln(os.Stderr, `Connecting to`, address, name)
	conn := h.Connect(address)
	fmt.Fprintln(os.Stderr, `Connected`)

	Stream(conn, Stdio)
}
