package server

import (
	"net"
	"net/textproto"
	"time"

	"github.com/j7b/smtpd/config"
	"github.com/j7b/smtpd/logging"
	"github.com/j7b/smtpd/maildir"
	"github.com/j7b/smtpd/server/session"
	"github.com/j7b/smtpd/types"
)

var log = logging.Logger

type server struct {
	cfg  config.Interface
	mdir maildir.Interface
}

func panics() {
	if r := recover(); r != nil {
		log.Println("PANIC: ", r)
	}
}

func (s *server) handle(c net.Conn) {
	defer panics()
	c.SetReadDeadline(time.Now().Add(10 * time.Second))
	tp := textproto.NewConn(c)
	err := tp.PrintfLine("220 %s", s.cfg.DefaultHost())
	if err != nil {
		tp.Close()
		return
	}
	ses := session.New(&types.NetConn{Conn: tp, C: c}, s.cfg, s.mdir)
	ses.Start()
}

// START OMIT

// Serve spawns handlers for connections.
func Serve(cfg config.Interface, mdir maildir.Interface, l net.Listener) (err error) {
	// END OMIT
	var c net.Conn
	s := &server{cfg: cfg, mdir: mdir}
	for {
		c, err = l.Accept()
		if err != nil {
			return
		}
		go s.handle(c)
	}
}
