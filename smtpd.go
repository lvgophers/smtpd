package main

import (
	"flag"
	"net"
	"os"
	"os/user"
	"path/filepath"

	"github.com/lvgophers/smtpd/config"
	"github.com/lvgophers/smtpd/logging"
	"github.com/lvgophers/smtpd/maildir"
	"github.com/lvgophers/smtpd/server"
)

var log = logging.Logger

func homedir() string {
	u, err := user.Current()
	if err != nil {
		panic(err)
	}
	return u.HomeDir
}

func getwd() string {
	d, err := os.Getwd()
	if err != nil {
		panic(err)
	}
	return d
}

var listenaddr = flag.String("addr", ":2525", "Listen address")
var configdir = flag.String("config", filepath.Join(homedir(), ".smtpd"), "Configuration directory")
var mdir = flag.String("maildir", getwd(), "Maildir directory")

func main() {
	flag.Parse()
	conf, err := config.New(*configdir)
	if err != nil {
		log.Fatal(err)
	}
	maild, err := maildir.New(*mdir)
	if err != nil {
		log.Fatal(err)
	}
	l, err := net.Listen("tcp",*listenaddr)
	if err != nil {
		log.Fatal(err)
	}
	log.Fatal(server.Serve(conf, maild, l))
}
