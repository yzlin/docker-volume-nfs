package driver

import (
	"fmt"
	"os"
	"path/filepath"
	"sync"

	log "github.com/Sirupsen/logrus"
	"github.com/docker/go-plugins-helpers/volume"
)

const (
	nfsOpts          = "nfsopts"
	nfsV3DefaultOpts = "port=2049,nolock,proto=tcp"
)

type mount struct {
	name        string
	mountpoint  string
	connections int
	opts        map[string]string
}

type nfsDriver struct {
	m       *sync.RWMutex
	root    string
	mounts  map[string]*mount
	version int
	nfsOpts map[string]string
}

// NewNFSDriver creates a NFS driver which implements docker's volume plugin API
func NewNFSDriver(root string, version int, nfsOpts string) nfsDriver {
	d := nfsDriver{
		m:       &sync.RWMutex{},
		root:    root,
		mounts:  map[string]*mount{},
		version: version,
		nfsOpts: map[string]string{},
	}

	if nfsOpts != "" {
		d.nfsOpts[nfsOpts] = nfsOpts
	}
	return d
}

func (n nfsDriver) Create(r volume.Request) volume.Response {
	logger := log.WithField("method", "create")
	logger.Debugf("%#v", r)

	n.m.Lock()
	defer n.m.Unlock()

	name := r.Name

	if _, ok := r.Options[srcOpt]; !ok {
		err := fmt.Errorf(`"%s" option should not be empty`, srcOpt)
		logger.Errorf(err.Error())
		return volume.Response{Err: err.Error()}
	}

	mountpoint := filepath.Join(n.root, name)
	logger.Debugf("Create volume %s: mountpoint=%s, opts=%#v", name, mountpoint, r.Options)

	if err := createDest(mountpoint); err != nil {
		logger.Errorf("Failed to create mountpoint: %s", err)
		return volume.Response{Err: err.Error()}
	}

	mnt, ok := n.mounts[name]
	if ok && mnt.connections > 0 {
		mnt.opts = r.Options
	} else {
		mnt = &mount{
			name:        name,
			mountpoint:  mountpoint,
			opts:        r.Options,
			connections: 0,
		}
	}

	n.mounts[name] = mnt

	return volume.Response{}
}

func (n nfsDriver) Remove(r volume.Request) volume.Response {
	logger := log.WithField("method", "remove")
	logger.Debugf("%#v", r)

	n.m.Lock()
	defer n.m.Unlock()

	name := r.Name

	if mnt, ok := n.mounts[name]; ok {
		if mnt.connections > 0 {
			err := fmt.Errorf("Volume %s is currently in use", name)
			logger.Errorf(err.Error())
			return volume.Response{Err: err.Error()}
		}
		delete(n.mounts, name)
	}

	return volume.Response{}
}

func (n nfsDriver) Path(r volume.Request) volume.Response {
	logger := log.WithField("method", "path")
	logger.Debugf("%#v", r)

	name := r.Name
	mountpoint := filepath.Join(n.root, name)
	logger.Debugf("Volume %s with path: %s", name, mountpoint)
	return volume.Response{Mountpoint: mountpoint}
}

func (n nfsDriver) Get(r volume.Request) volume.Response {
	logger := log.WithField("method", "get")
	logger.Debugf("%#v", r)

	n.m.RLock()
	defer n.m.RUnlock()

	name := r.Name

	if mnt, ok := n.mounts[name]; ok {
		log.Debugf("Mount found for %s: mountpoint=%s", name, mnt.mountpoint)
		return volume.Response{Volume: &volume.Volume{Name: name, Mountpoint: mnt.mountpoint}}
	}
	return volume.Response{}
}

func (n nfsDriver) List(r volume.Request) volume.Response {
	logger := log.WithField("method", "list")
	logger.Debugf("%#v", r)

	n.m.RLock()
	defer n.m.RUnlock()

	volumes := []*volume.Volume{}
	for _, mnt := range n.mounts {
		volumes = append(volumes, &volume.Volume{
			Name:       mnt.name,
			Mountpoint: mnt.mountpoint,
		})
	}

	logger.Debugf("volumes: %#v", volumes)
	return volume.Response{Volumes: volumes}
}

func (n nfsDriver) Capabilities(r volume.Request) volume.Response {
	logger := log.WithField("method", "capabilities")
	logger.Debugf("%#v", r)

	return volume.Response{
		Capabilities: volume.Capability{
			Scope: "locale",
		},
	}
}

func (n nfsDriver) Mount(r volume.MountRequest) volume.Response {
	logger := log.WithField("method", "mount")
	logger.Debugf("%#v", r)

	n.m.Lock()
	defer n.m.Unlock()

	name := r.Name

	mnt, ok := n.mounts[name]
	if !ok {
		err := fmt.Errorf("Volume %s doesn't exist", name)
		logger.Errorf(err.Error())
		return volume.Response{Err: err.Error()}
	}

	source, ok := mnt.opts[srcOpt]
	if !ok || source == "" {
		err := fmt.Errorf("NFS source option (src) is not provided")
		logger.Errorf("Failed to mount NFS volume: %s", err)
		return volume.Response{Err: err.Error()}
	}

	if mnt.connections > 0 {
		logger.Infof("Using existing NFS volume mount: %s", mnt.mountpoint)
		if err := run("grep", "-c", mnt.mountpoint, "/proc/mounts"); err == nil {
			mnt.connections++
			return volume.Response{Mountpoint: mnt.mountpoint}
		}
		logger.Info("Existing NFS volume not mounted, force remount.")
	}

	logger.Infof("Mounting NFS volume %s on %s", source, mnt.mountpoint)

	if err := createDest(mnt.mountpoint); err != nil {
		return volume.Response{Err: err.Error()}
	}

	if err := n.mountVolume(mnt, source, n.version); err != nil {
		return volume.Response{Err: err.Error()}
	}

	mnt.connections++

	return volume.Response{Mountpoint: mnt.mountpoint}
}

func (n nfsDriver) Unmount(r volume.UnmountRequest) volume.Response {
	logger := log.WithField("method", "unmount")
	logger.Debugf("%#v", r)

	n.m.Lock()
	defer n.m.Unlock()

	name := r.Name

	mnt, ok := n.mounts[name]
	if !ok {
		err := fmt.Errorf("Volume %s doesn't exist", name)
		logger.Errorf(err.Error())
		return volume.Response{Err: err.Error()}
	}

	if mnt.connections > 1 {
		logger.Infof("Skipping unmount for %s - in use by other containers", name)
		mnt.connections--
		return volume.Response{}
	}

	logger.Infof("Unmounting volume name %s from %s", name, mnt.mountpoint)

	if err := run("umount", mnt.mountpoint); err != nil {
		logger.Errorf("Error unmounting volume: %s", err.Error())
		return volume.Response{Err: err.Error()}
	}

	mnt.connections--

	// Cleanup
	if empty, _ := isEmpty(mnt.mountpoint); !empty {
		logger.Warnf("Directory %s is not empty after unmount. Skipping RemoveAll call.", mnt.mountpoint)
	} else if err := os.RemoveAll(mnt.mountpoint); err != nil {
		return volume.Response{Err: err.Error()}
	}

	return volume.Response{}
}

func (n *nfsDriver) mountVolume(mnt *mount, src string, version int) error {
	var args []string

	options := mnt.opts
	if options == nil {
		options = n.nfsOpts
	} else {
		options = merge(options, n.nfsOpts)
	}

	opts := ""
	if val, ok := options[nfsOpts]; ok {
		opts = val
	}

	mountCmd := "mount"

	if log.GetLevel() == log.DebugLevel {
		args = append(args, "-v")
	}

	switch version {
	case 3:
		log.Debugf("Mounting with NFSv3 - src: %s, dst: %s", src, mnt.mountpoint)
		if opts == "" {
			opts = nfsV3DefaultOpts
		}
		args = append(args, "-t", "nfs", "-o", opts, src, mnt.mountpoint)
	default:
		log.Debugf("Mounting with NFSv4 - src: %s, dst: %s", src, mnt.mountpoint)
		if opts != "" {
			args = append(args, "-t", "nfs4", "-o", opts, src, mnt.mountpoint)
		} else {
			args = append(args, "-t", "nfs4", src, mnt.mountpoint)
		}
	}

	return run(mountCmd, args...)
}
