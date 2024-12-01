//go:build linux

package main

import (
	"bytes"
	"io"
	"log"
	"net"
	"sync"
	"time"

	"tinygo.org/x/bluetooth"
)

type Device struct {
	adapter *bluetooth.Adapter
	rx, tx  *bluetooth.Characteristic

	mu      sync.Mutex
	rxBuf   *bytes.Buffer
	rxBufCh chan struct{}
}

func NewDevice() *Device {
	d := &Device{
		adapter: bluetooth.DefaultAdapter,
		rx:      &bluetooth.Characteristic{},
		tx:      &bluetooth.Characteristic{},
		rxBuf:   bytes.NewBuffer(nil),
		rxBufCh: make(chan struct{}),
	}

	Must(d.adapter.Enable())

	service := bluetooth.Service{
		UUID: uuidService,
		Characteristics: []bluetooth.CharacteristicConfig{
			{
				Handle: d.rx,
				UUID:   uuidRx,
				Flags:  bluetooth.CharacteristicWritePermission | bluetooth.CharacteristicWriteWithoutResponsePermission,
				WriteEvent: func(client bluetooth.Connection, offset int, value []byte) {
					d.mu.Lock()
					defer d.mu.Unlock()
					d.rxBuf.Write(value)
					select {
					case d.rxBufCh <- struct{}{}:
					default:
					}
				},
			},
			{
				Handle: d.tx,
				UUID:   uuidTx,
				Flags:  bluetooth.CharacteristicNotifyPermission | bluetooth.CharacteristicReadPermission,
			},
		},
	}

	Must(d.adapter.AddService(&service))

	return d
}

func (d *Device) Address() string {
	return Must1(d.adapter.Address()).String()
}

func (d *Device) StartAdvertisement() {
	a := d.adapter.DefaultAdvertisement()
	Must(a.Configure(bluetooth.AdvertisementOptions{
		ServiceUUIDs: []bluetooth.UUID{uuidService},
	}))
	Must(a.Start())
}

func (d *Device) Write(p []byte) (int, error) {
	// log.Println(`Write:`, len(p))
	count := 0
	for len(p) > 0 {
		n, err := d.tx.Write(p[:Min(64, len(p))])
		if err != nil {
			return count, err
		}
		p = p[n:]
		count += n
	}
	return count, nil
}

func (d *Device) Read(p []byte) (int, error) {
	d.mu.Lock()
	if d.rxBuf.Len() <= 0 {
		d.mu.Unlock()
		<-d.rxBufCh
		d.mu.Lock()
	}
	n, err := d.rxBuf.Read(p)
	d.mu.Unlock()
	// log.Println(`Read:`, p[:n], err)
	return n, err
}

func main() {
	log.SetFlags(log.Flags() | log.Lshortfile)
	d := NewDevice()
	log.Println(`Address:`, d.Address())
	d.StartAdvertisement()

	conn := Must1(net.Dial(`tcp4`, `localhost:22`))

	go func() {
		Must1(io.Copy(conn, d))
		log.Println(`exited`)
	}()

	time.Sleep(time.Second * 10)
	log.Println(`Start streaming`)

	Must1(io.Copy(d, conn))
}
