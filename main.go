package main

import (
	"os"

	log "github.com/Sirupsen/logrus"
	"github.com/docker/go-plugins-helpers/volume"

	"docker-volume-nfs/driver"
)

func main() {
	log.SetLevel(log.DebugLevel)
	if file, err := os.OpenFile("/var/log/nfs.log", os.O_CREATE|os.O_WRONLY, 0666); err == nil {
		log.SetOutput(file)
	} else {
		log.Fatal("Failed to open log file")
	}

	d := driver.NewNFSDriver("/mnt/fs", 3, "")
	h := volume.NewHandler(d)
	log.Error(h.ServeUnix("/run/docker/plugins/nfs.sock", 0))
}
