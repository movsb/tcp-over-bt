//go:build darwin

package main

import (
	"log"
	"time"

	"tinygo.org/x/bluetooth"
)

func main() {
	adapter := bluetooth.DefaultAdapter
	if err := adapter.Enable(); err != nil {
		log.Fatalln(err)
	}

	adapter.SetConnectHandler(func(device bluetooth.Device, connected bool) {
		log.Println(`OnConnect:`, device, connected)
	})
	addr := bluetooth.Address{}
	addr.Set(`abe46b44-7f25-a25f-c915-b7b4839d39e7`)
	device, err := adapter.Connect(addr, bluetooth.ConnectionParams{})
	if err != nil {
		log.Fatalln(err)
	}
	log.Println(`Device:`, device.Address)
	services, err := device.DiscoverServices([]bluetooth.UUID{uuidService})
	if err != nil {
		log.Fatalln(err)
	}
	svc := services[0]
	chs, err := svc.DiscoverCharacteristics([]bluetooth.UUID{uuidTx, uuidRx})
	if err != nil {
		log.Fatalln(err)
	}
	chTx, chRx := chs[0], chs[1]
	chTx.EnableNotifications(func(buf []byte) {
		log.Println(`Received:`, string(buf))
	})
	chRx.Write([]byte(`send from host`))
	time.Sleep(time.Minute)
}
