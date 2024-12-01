//go:build linux

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

	adver := adapter.DefaultAdvertisement()
	if err := adver.Configure(bluetooth.AdvertisementOptions{
		LocalName:    `My Zero`,
		ServiceUUIDs: []bluetooth.UUID{uuidService},
	}); err != nil {
		log.Fatalln(err)
	}
	if err := adver.Start(); err != nil {
		log.Fatalln(err)
	}

	var rx, tx bluetooth.Characteristic
	if err := adapter.AddService(&bluetooth.Service{
		UUID: uuidService,
		Characteristics: []bluetooth.CharacteristicConfig{
			{
				Handle: &rx,
				UUID:   uuidRx,
				Flags:  bluetooth.CharacteristicWritePermission | bluetooth.CharacteristicWriteWithoutResponsePermission,
				WriteEvent: func(client bluetooth.Connection, offset int, value []byte) {
					log.Println(`Received:`, string(value))
				},
			},
			{
				Handle: &tx,
				UUID:   uuidTx,
				Flags:  bluetooth.CharacteristicNotifyPermission | bluetooth.CharacteristicReadPermission,
			},
		},
	}); err != nil {
		log.Fatalln(err)
	}

	log.Println(adapter.Address())

	time.Sleep(time.Second * 5)

	if _, err := tx.Write([]byte(`send from device`)); err != nil {
		log.Fatalln(err)
	}

	time.Sleep(time.Minute)
}
