package types

import (
	"net"
	"net/textproto"
)

// NetConn is a textproto.Conn and the underlying connection
type NetConn struct {
	*textproto.Conn
	C net.Conn
}
