package session

import (
	"fmt"
	"io"
	"io/ioutil"
	"math/rand"
	"net"
	"net/mail"
	"net/textproto"
	"os"
	"path/filepath"
	"runtime/debug"
	"strings"
	"time"

	"github.com/j7b/smtpd/config"
	"github.com/j7b/smtpd/logging"
	"github.com/j7b/smtpd/maildir"
	"github.com/j7b/smtpd/types"
)

func init() {
	rand.Seed(time.Now().UnixNano())
}

type errstr string

func (e errstr) Error() string {
	return string(e)
}

var log = logging.Logger

var code211 = &textproto.Error{Code: 211, Msg: "System status, or system help reply"}
var code214 = &textproto.Error{Code: 214, Msg: "Help message"}
var code220 = &textproto.Error{Code: 220, Msg: "Service ready"}
var code221 = &textproto.Error{Code: 221, Msg: "Service closing transmission channel"}
var code250 = &textproto.Error{Code: 250, Msg: "Requested mail action okay, completed"}
var code251 = &textproto.Error{Code: 251, Msg: "User not local"}
var code354 = &textproto.Error{Code: 354, Msg: "Start mail input; end with <CRLF>.<CRLF>"}
var code421 = &textproto.Error{Code: 421, Msg: "Service not available, closing transmission channel"}
var code450 = &textproto.Error{Code: 450, Msg: "Requested mail action not taken: mailbox unavailable"}
var code451 = &textproto.Error{Code: 451, Msg: "Requested action aborted: error in processing"}
var code452 = &textproto.Error{Code: 452, Msg: "Requested action not taken: insufficient system storage"}
var code500 = &textproto.Error{Code: 500, Msg: "Syntax error, command unrecognized"}
var code501 = &textproto.Error{Code: 501, Msg: "Syntax error in parameters or arguments"}
var code502 = &textproto.Error{Code: 502, Msg: "Command not implemented"}
var code503 = &textproto.Error{Code: 503, Msg: "Bad sequence of commands"}
var code504 = &textproto.Error{Code: 504, Msg: "Command parameter not implemented"}
var code550 = &textproto.Error{Code: 550, Msg: "Requested action not taken: mailbox unavailable"}
var code551 = &textproto.Error{Code: 551, Msg: "User not local"}
var code552 = &textproto.Error{Code: 552, Msg: "Requested mail action aborted: exceeded storage allocation"}
var code553 = &textproto.Error{Code: 553, Msg: "Requested action not taken: mailbox name not allowed"}
var code554 = &textproto.Error{Code: 554, Msg: "Transaction failed"}

var toomanyrcpt = &textproto.Error{Code: 452, Msg: "too many recipients"}
var norelay = &textproto.Error{Code: 553, Msg: "no relay"}

func (s *session) panic() {
	defer s.Close()
	if r := recover(); r != nil {
		if tpe, ok := r.(*textproto.Error); ok {
			if tpe.Code != 421 {
				s.PrintfLine(tpe.Error())
				s.PrintfLine(code421.Error())
			} else {
				s.PrintfLine(tpe.Error())
			}
			return
		}
		if oe, ok := r.(net.OpError); ok {
			if oe.Timeout() {
				s.PrintfLine("%v %s", 421, "timeout")
			}
			return
		}
		log.Println(r, string(debug.Stack()))
	}
}

func check(e error) {
	if e != nil {
		panic(e)
	}
}

func formatok(s string) bool {
	if strings.Index(s, "<") != 0 || strings.Index(s, ">") != len(s)-1 {
		return false
	}
	return true
}

func parseaddr(s string) (mailbox, domain string, err error) {
	addr, err := mail.ParseAddress(s)
	if err != nil {
		return
	}
	if addr.Name != "" {
		err = errstr("misparsed address")
		return
	}
	bareaddr := strings.Trim(addr.Address, "<>")
	parts := strings.SplitN(bareaddr, "@", 2)
	if len(parts) != 2 {
		err = errstr("ambiguous address")
		return
	}
	mailbox, domain = parts[0], parts[1]
	return
}

type session struct {
	*types.NetConn
	cfg  config.Interface
	mdir maildir.Interface
	helo string
	from string
	rcpt []string
}

func (s *session) hello(parts []string) (err error) {
	if s.helo != "" {
		return code503
	}
	if len(parts) != 2 {
		return code501
	}
	s.helo = parts[1]
	s.PrintfLine("%v Hello %s", 250, parts[1])
	return
}

func (s *session) vrfy(parts []string) error {
	s.PrintfLine("502 send some mail, see what happens")
	return nil
}

func (s *session) mailfrom(parts []string) (err error) {
	if s.helo == "" || s.from != "" {
		return code503
	}
	if len(parts) < 2 {
		return code501
	}
	newparts := strings.SplitN(strings.Join(parts[1:], " "), ":", 2)
	if len(newparts) != 2 {
		return code501
	}
	from, fromaddr := strings.TrimSpace(newparts[0]), strings.TrimSpace(newparts[1])
	if from != "from" {
		return code501
	}
	if !formatok(fromaddr) {
		panic(code501)
	}
	s.from = fromaddr
	s.PrintfLine("250 %s OK", fromaddr)
	return
}

func (s *session) rcptto(parts []string) (err error) {
	if s.helo == "" || s.from == "" {
		return code503
	}
	if len(parts) < 2 {
		return code501
	}
	if len(s.rcpt) == s.cfg.MaxRcpt() {
		return toomanyrcpt
	}
	newparts := strings.SplitN(strings.Join(parts[1:], " "), ":", 2)
	if len(newparts) != 2 {
		return code501
	}
	to, rcpt := strings.TrimSpace(newparts[0]), strings.TrimSpace(newparts[1])
	if to != "to" {
		return code501
	}
	if !formatok(rcpt) {
		return code501
	}
	mailbox, domain, err := parseaddr(rcpt)
	if err != nil {
		return code501
	}
	if !s.cfg.Host(domain) {
		return norelay
	}
	s.rcpt = append(s.rcpt, fmt.Sprintf("%s@%s", mailbox, domain))
	s.PrintfLine("250 %s OK", rcpt)
	return
}

func (s *session) data(parts []string) (err error) {
	if len(s.rcpt) == 0 || s.from == "" || s.helo == "" {
		return code503
	}
	if len(parts) != 1 {
		return code501
	}
	tf, err := ioutil.TempFile(s.mdir.TmpDir(),
		fmt.Sprintf("%v.%x.", time.Now().UnixNano(), rand.Int63())) // Should be collisionproof enough
	if err != nil {
		panic(code452)
	}
	defer tf.Close()
	check(s.PrintfLine(code354.Error()))
	r := s.DotReader()
	_, err = io.CopyN(tf, r, s.cfg.MaxSize())
	if err == nil {
		os.Remove(tf.Name())
		panic(code552)
	}
	if err != nil && err != io.EOF {
		os.Remove(tf.Name())
		panic(err)
	}
	tf.Close()
	basename := filepath.Base(tf.Name())
	err = os.Rename(tf.Name(), filepath.Join(s.mdir.NewDir(), basename))
	if err != nil {
		os.Remove(tf.Name())
		panic(code452)
	}
	s.PrintfLine("250 dirdel (%s)", basename)
	s.from = ""
	s.rcpt = nil
	return
}

func (s *session) rset(parts []string) (err error) {
	if len(parts) != 1 {
		return code501
	}
	s.from = ""
	s.rcpt = nil
	s.PrintfLine("250 OK")
	return
}

// Interface is the interface to a greeted SMTP session.
type Interface interface {
	Start() // Starts the mail session.
}

// New returns a mail session Interface.
func New(c *types.NetConn, cfg config.Interface, mdir maildir.Interface) Interface {
	return &session{NetConn: c, cfg: cfg, mdir: mdir}
}

func (s *session) Start() {
	defer s.Close()
	defer s.panic()
	for {
		// s.C.SetReadDeadline(time.Now().Add(s.cfg.Timeout()))
		cmd, err := s.ReadLine()
		check(err)
		parts := strings.Split(strings.ToLower(cmd), " ")
		switch parts[0] {
		case "ehlo", "helo":
			err = s.hello(parts)
		case "mail":
			err = s.mailfrom(parts)
		case "rcpt":
			err = s.rcptto(parts)
		case "data":
			err = s.data(parts)
		case "rset":
			err = s.rset(parts)
		case "vrfy":
			err = s.vrfy(parts)
		case "help":
			s.PrintfLine("211 https://tools.ietf.org/html/rfc821")
		case "noop":
			s.PrintfLine("250 NOOP")
		case "quit":
			s.PrintfLine("%s", code221.Error())
			return
		default:
			err = code500
		}
		if err != nil {
			s.PrintfLine("%s", err.Error())
		}
	}
}
