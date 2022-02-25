package unixmsg

import (
	"fmt"
	"net"
	"syscall"
)

// https://github.com/mindreframer/golang-stuff/blob/master/github.com/youtube/vitess/go/umgmt/fdpass.go
// see also TestPassFD

func SendFd(conn *net.UnixConn, fd uintptr) error {
	rights := syscall.UnixRights(int(fd))
	dummy := []byte("x")
	n, oobn, err := conn.WriteMsgUnix(dummy, rights, nil)
	if err != nil {
		return fmt.Errorf("err %v", err)
	}
	if n != len(dummy) {
		return fmt.Errorf("short write %v", conn)
	}
	if oobn != len(rights) {
		return fmt.Errorf("short oob write %v", conn)
	}
	return nil
}

func RecvFd(conn *net.UnixConn) (uintptr, error) {
	buf := make([]byte, 32)
	oob := make([]byte, 32)
	_, oobn, _, _, err := conn.ReadMsgUnix(buf, oob)
	if err != nil {
		return 0, err
	}
	scms, err := syscall.ParseSocketControlMessage(oob[:oobn])
	if err != nil {
		return 0, fmt.Errorf("ParseSocketControlMessage %w", err)
	}
	if len(scms) != 1 {
		return 0, fmt.Errorf("SocketControlMessage count not 1: %v", len(scms))
	}
	scm := scms[0]
	fds, err := syscall.ParseUnixRights(&scm)
	if err != nil {
		return 0, fmt.Errorf("ParseUnixRights: %w", err)
	}
	if len(fds) != 1 {
		return 0, fmt.Errorf("fd count not 1: %v", len(fds))
	}
	return uintptr(fds[0]), nil
}
