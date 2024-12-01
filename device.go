//go:build linux

package main

import (
	"bytes"
	"io"
	"log"
	"net"
	"sync"

	"tinygo.org/x/bluetooth"
)

type Device struct {
	adapter *bluetooth.Adapter
	rx, tx  *bluetooth.Characteristic

	mu      sync.Mutex
	rxBuf   *bytes.Buffer
	rxBufCh chan struct{}

	firstMessage chan struct{}
}

func NewDevice() *Device {
	d := &Device{
		adapter: bluetooth.DefaultAdapter,
		rx:      &bluetooth.Characteristic{},
		tx:      &bluetooth.Characteristic{},
		rxBuf:   bytes.NewBuffer(nil),
		rxBufCh: make(chan struct{}),

		firstMessage: make(chan struct{}),
	}

	// TODO: not work
	// https://github.com/tinygo-org/bluetooth/issues/290
	d.adapter.SetConnectHandler(func(device bluetooth.Device, connected bool) {
		log.Println(`Connect:`, device, connected)
	})

	Must(d.adapter.Enable())

	service := bluetooth.Service{
		UUID: uuidService,
		Characteristics: []bluetooth.CharacteristicConfig{
			{
				Handle: d.rx,
				UUID:   uuidRx,
				Flags:  bluetooth.CharacteristicWritePermission | bluetooth.CharacteristicWriteWithoutResponsePermission,
				WriteEvent: func(client bluetooth.Connection, offset int, value []byte) {
					if d.firstMessage != nil {
						close(d.firstMessage)
						d.firstMessage = nil
						return
					}
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
	return splitWrite(d.tx, p)
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
	return n, err
}

// Because the ConnectHandler on Linux does not work,
// We treat first incoming message sent from Host as a greeting for Connect.
func (d *Device) WaitForConnection() {
	<-d.firstMessage
}

func main() {
	log.SetFlags(log.Flags() | log.Lshortfile)

	d := NewDevice()
	log.Println(`Address:`, d.Address())
	log.Println(`Start advertisement`)
	d.StartAdvertisement()

	log.Println(`Waiting for Connection`)
	d.WaitForConnection()
	log.Println(`Connected`)

	conn := Must1(net.Dial(`tcp4`, `localhost:22`))

	go func() {
		Must1(io.Copy(conn, d))
	}()

	Must1(io.Copy(d, conn))
}
