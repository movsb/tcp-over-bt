package main

import (
	"log"

	"tinygo.org/x/bluetooth"
)

func Must[T any](t T, err error) T {
	if err != nil {
		log.Fatalln(err)
	}
	return t
}

var (
	uuidService = Must(bluetooth.ParseUUID(`923BFB18-A711-4923-82A8-988AD38AF7C1`))
	uuidTx      = Must(bluetooth.ParseUUID(`3F33F755-C5A9-4F25-B0CA-8BFFEF13B905`))
	uuidRx      = Must(bluetooth.ParseUUID(`41F94EAB-B906-4AE6-BEB9-0D5CA55EC4CB`))
)
