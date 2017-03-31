package driver

import (
	"fmt"
	"io"
	"os"
	"os/exec"

	log "github.com/Sirupsen/logrus"
)

const (
	srcOpt = "src"
)

func createDest(dest string) error {
	fi, err := os.Lstat(dest)

	if os.IsNotExist(err) {
		if err := os.MkdirAll(dest, 0755); err != nil {
			return err
		}
	} else if err != nil {
		return err
	}

	if fi != nil && !fi.IsDir() {
		return fmt.Errorf("%v already exist and it's not a directory", dest)
	}
	return nil
}

func run(name string, arg ...string) error {
	log.Debugf("exec: %s %v", name, arg)
	if out, err := exec.Command(name, arg...).CombinedOutput(); err != nil {
		log.Infof(string(out))
		return err
	}
	return nil
}

func merge(src, src2 map[string]string) map[string]string {
	if len(src) == 0 && len(src2) == 0 {
		return map[string]string{}
	}

	dst := map[string]string{}
	for k, v := range src2 {
		dst[k] = v
	}
	for k, v := range src {
		dst[k] = v
	}
	return dst
}

func isEmpty(name string) (bool, error) {
	f, err := os.Open(name)
	if err != nil {
		return false, nil
	}
	defer f.Close()

	_, err = f.Readdirnames(1)
	if err == io.EOF {
		return true, nil
	}
	return false, err
}
