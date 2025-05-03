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

	rx, tx *bluetooth.Characteristic

	// Currently, the SetConnectHandler on Linux does not work,
	// Hence we do not know when our device is connected or disconnected.
	// The control characteristic is sent from Host to let us know that
	// a new connection is being made.
	ctrl       *bluetooth.Characteristic
	connection chan struct{}

	// To close a connection, close this channel.
	closed chan struct{}

	mu      sync.Mutex
	rxBuf   *bytes.Buffer
	rxBufCh chan struct{}
}

func NewDevice() *Device {
	d := &Device{
		adapter: bluetooth.DefaultAdapter,
		rx:      &bluetooth.Characteristic{},
		tx:      &bluetooth.Characteristic{},

		ctrl:       &bluetooth.Characteristic{},
		connection: make(chan struct{}),
		closed:     make(chan struct{}),

		rxBuf:   bytes.NewBuffer(nil),
		rxBufCh: make(chan struct{}),
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
				Handle:     d.rx,
				UUID:       uuidRx,
				Flags:      bluetooth.CharacteristicWritePermission,
				WriteEvent: d.onRecv,
			},
			{
				Handle: d.tx,
				UUID:   uuidTx,
				Flags:  bluetooth.CharacteristicNotifyPermission | bluetooth.CharacteristicReadPermission,
			},
			{
				Handle:     d.ctrl,
				UUID:       uuidCtrl,
				Flags:      bluetooth.CharacteristicWritePermission,
				WriteEvent: d.writeControl,
			},
		},
	}

	Must(d.adapter.AddService(&service))

	a := d.adapter.DefaultAdvertisement()
	Must(a.Configure(bluetooth.AdvertisementOptions{
		ServiceUUIDs: []bluetooth.UUID{uuidService},
	}))

	return d
}

func (d *Device) writeControl(client bluetooth.Connection, offset int, p []byte) {
	close(d.closed)

	d.connection <- struct{}{}
}

func (d *Device) onRecv(client bluetooth.Connection, offset int, p []byte) {
	d.mu.Lock()
	defer d.mu.Unlock()
	d.rxBuf.Write(p)
	select {
	case d.rxBufCh <- struct{}{}:
	default:
	}
}

// The hardware address.
// NOTE: It may be different from what MacOS shows.
func (d *Device) Address() string {
	return Must1(d.adapter.Address()).String()
}

// 超时控制默认为“0”，即不超时，永远广播。
// https://github.com/tinygo-org/bluetooth/blob/a668e1b0a062612faa41ac354f7edd5b25428101/gap_linux.go#L79-L84
func (d *Device) Listen() {
	a := d.adapter.DefaultAdvertisement()
	Must(a.Start())
}

func (d *Device) Accept() Conn {
	<-d.connection
	d.closed = make(chan struct{})
	d.rxBuf.Reset()

	conn := &DeviceConn{
		d: d,
		w: NewSegmentedWriter(d.tx, maxPacketSize),
	}

	return conn
}

type DeviceConn struct {
	d *Device
	w io.Writer
}

func (c *DeviceConn) Write(p []byte) (int, error) {
	select {
	case <-c.d.closed:
		return 0, errConnClosed
	default:
	}
	return c.w.Write(p)
}

func (c *DeviceConn) Read(p []byte) (int, error) {
	c.d.mu.Lock()

	if c.d.rxBuf.Len() <= 0 {
		c.d.mu.Unlock()
		select {
		case <-c.d.rxBufCh:
		case <-c.d.closed:
			return 0, errConnClosed
		}
		c.d.mu.Lock()
	}

	n, err := c.d.rxBuf.Read(p)
	c.d.mu.Unlock()
	return n, err
}

func main() {
	log.SetFlags(log.Flags() | log.Lshortfile)

	d := NewDevice()
	log.Println(`Address:`, d.Address())

	// 库代码硬编码成了无超时，所以只需要调用一次。
	log.Println(`Start advertisement`)
	d.Listen()

	for {

		log.Println(`Waiting for Connection`)
		conn := d.Accept()
		log.Println(`Connected`)

		remote := Must1(net.Dial(`tcp4`, `localhost:22`))

		go func() {
			<-d.closed
			remote.Close()
		}()

		Stream(conn, remote)
	}
}
