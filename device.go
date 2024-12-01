//go:build linux

package main

import (
	"bytes"
	"io"
	"log"
	"net"
	"time"

	"tinygo.org/x/bluetooth"
)

type Device struct {
	adapter *bluetooth.Adapter
	rx, tx  *bluetooth.Characteristic
	chR     chan []byte
	rxBuf   *bytes.Buffer
}

func NewDevice() *Device {
	d := &Device{
		adapter: bluetooth.DefaultAdapter,
		rx:      &bluetooth.Characteristic{},
		tx:      &bluetooth.Characteristic{},
		chR:     make(chan []byte),
		rxBuf:   bytes.NewBuffer(nil),
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
					d.chR <- value
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
			return 0, err
		}
		p = p[n:]
		count += n
	}
	return count, nil
}

func (d *Device) Read(p []byte) (int, error) {
	if d.rxBuf.Len() <= 0 {
		next := <-d.chR
		d.rxBuf.Write(next)
	}
	n, err := d.rxBuf.Read(p)
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
