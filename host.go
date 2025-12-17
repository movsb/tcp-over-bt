//go:build darwin

package main

import (
	"context"
	"encoding/binary"
	"fmt"
	"log"
	"os"
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

	// try to test max packet size
	fmt.Fprintln(os.Stderr, `Testing MTU...`)
	mtu := h.testMTU(ctrl)
	if mtu <= 0 {
		log.Fatalln(`cannot get mtu size`)
	}
	fmt.Fprintln(os.Stderr, `MTU:`, mtu)

	w := NewSegmentedWriter(NewOrderedWriter(rx), mtu-SeqLen)
	r := NewOrderedReader(context.Background())

	// must be enabled before open new connection, or there
	// will be packet lose.
	Must(tx.EnableNotifications(func(buf []byte) {
		if err := r.Receive(buf); err != nil {
			log.Fatalln(err)
		}
	}))

	// open a new connection with specified mtu.
	ctrlBuf := [5]byte{}
	ctrlBuf[0] = byte(NewConn)
	binary.LittleEndian.PutUint32(ctrlBuf[1:], uint32(mtu))
	Must1(ctrl.Write(ctrlBuf[:]))

	return &ReadWriter{Writer: w, Reader: r}
}

func (h *Host) testMTU(ctrl bluetooth.DeviceCharacteristic) int {
	base := 64
	c := 1
	for {
		b := make([]byte, base*c)
		b[0] = byte(TestMTU)
		_, err := ctrl.Write(b)
		if err != nil {
			return base * (c - 1)
		}
		c++
	}
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
