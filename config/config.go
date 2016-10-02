package config

import (
	"bufio"
	"fmt"
	"os"
	"path/filepath"
	"sync"
	"time"

	"github.com/j7b/smtpd/logging"
)

// Default constants.
const (
	DefaultTimeout = 10 * time.Second
	DefaultMaxRcpt = 10
	DefaultMaxSize = 1 * 1024 * 1024 // mpegabyte
)

// START OMIT
// Interface is the method set for parsed config files.
type Interface interface {
	Host(name string) bool
	Reload() (err error)
	DefaultHost() string
	Timeout() time.Duration
	MaxRcpt() int
	MaxSize() int64
}

// END OMIT

type dir struct {
	l           sync.RWMutex
	configdir   string
	defaulthost string
	rcpthosts   []string
}

func (d *dir) Timeout() time.Duration {
	return DefaultTimeout
}

func (d *dir) MaxRcpt() int {
	return DefaultMaxRcpt
}

func (d *dir) MaxSize() int64 {
	return DefaultMaxSize
}

func (d *dir) lock() {
	d.l.Lock()
}

func (d *dir) unlock() {
	d.l.Unlock()
}

func (d *dir) rlock() {
	d.l.RLock()
}

func (d *dir) runlock() {
	d.l.RUnlock()
}

func (d *dir) rhosts() (err error) {
	d.lock()
	defer d.unlock()
	conffile := filepath.Join(d.configdir, "rcpthosts")
	f, err := os.Open(conffile)
	if err != nil {
		if os.IsNotExist(err) {
			return fmt.Errorf("rcpthosts not found")
		}
		return err
	}
	defer f.Close()
	d.rcpthosts = nil
	scanner := bufio.NewScanner(f)
	for scanner.Scan() {
		if scanner.Text() != "" {
			d.rcpthosts = append(d.rcpthosts, scanner.Text())
		}
	}
	l := len(d.rcpthosts)
	if l == 0 {
		return fmt.Errorf("no rctphosts")
	}
	if err = scanner.Err(); err == nil {
		if d.defaulthost != "" {
			d.rcpthosts = append(d.rcpthosts, d.defaulthost)
			d.rcpthosts[0], d.rcpthosts[l] = d.rcpthosts[l], d.rcpthosts[0]
		}
	}
	return
}

func (d *dir) defaulthosts() (err error) {
	d.lock()
	defer d.unlock()
	f, err := os.Open(filepath.Join(d.configdir, "defaulthost"))
	if err != nil {
		return
	}
	scanner := bufio.NewScanner(f)
	if scanner.Scan() {
		if host := scanner.Text(); host != "" {
			d.defaulthost = host
		}
	}
	return
}

func (d *dir) Reload() error {
	d.defaulthosts()
	return d.rhosts()
}

func (d *dir) Host(name string) bool {
	d.rlock()
	defer d.runlock()
	for _, h := range d.rcpthosts {
		if h == name {
			return true
		}
	}
	return false
}

func (d *dir) DefaultHost() string {
	if d.defaulthost != "" {
		return d.defaulthost
	}
	return d.rcpthosts[0]
}

// New returns a config interface.
func New(configdir string) (i Interface, err error) {
	fi, err := os.Stat(configdir)
	if err != nil {
		return
	}
	if !fi.IsDir() {
		return nil, fmt.Errorf("not a directory: %s", configdir)
	}
	d := dir{configdir: configdir}
	err = d.defaulthosts()
	if err != nil {
		logging.Logger.Println("Defaulthost not found, will use first rcpthost in greeting.")
	}
	if err = d.rhosts(); err != nil {
		return
	}
	return &d, nil
}
