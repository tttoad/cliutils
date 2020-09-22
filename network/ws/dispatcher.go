// +build linux

package ws

import (
	"fmt"
	"net"
	"time"

	"github.com/gobwas/ws/wsutil"

	"gitlab.jiagouyun.com/cloudcare-tools/cliutils/system/rtpanic"
)

var (
	CommonChanCap = 128
)

type Cli struct {
	conn          net.Conn
	id            string
	born          time.Time
	lastHeartbeat time.Time
}

func (s *Server) dispatcher() {
	var f rtpanic.RecoverCallback

	tick := time.NewTicker(time.Minute)
	defer tick.Stop()

	f = func(trace []byte, _ error) {
		defer rtpanic.Recover(f, nil)

		if trace != nil {
			l.Warnf("recover ok: %s", string(trace))
		}

		for {
			select {
			case cli := <-s.wscliCh: // new ws connection comming
				if cli != nil {
					l.Debugf("%s add datakit %s(from %s)", s.Bind, cli.id, cli.conn.RemoteAddr().String())
					s.clis[cli.id] = cli
				}

			case msg := <-s.sendMsgCh: // send ws msg to cli
				s.doSendMsgToClient(msg.msg, msg.to...)

			case cliid := <-s.hbCh: // cli heartbeat comming
				if cli, ok := s.clis[cliid]; ok {
					l.Debugf("update heartbeat on %s", cliid)
					cli.lastHeartbeat = time.Now()
				} else {
					l.Warnf("cliid %s not found", cliid)
				}

			case <-tick.C:
				// TODO:
				//  - clear expired dmsg
				//  - clear ws cli without heartbeat
				//  - ...
				l.Infof("total clients: %d", len(s.clis))
			case <-s.exit.Wait():
				for _, c := range s.clis {
					if err := c.conn.Close(); err != nil {
						l.Warn("c.conn.Close(): %s, ignored", err.Error())
					}
				}

				l.Info("dispatcher exit.")
				return
			}
		}
	}

	f(nil, nil)
}

func todo() {
	panic(fmt.Errorf("not implement"))
}

func (s *Server) doSendMsgToClient(msg []byte, to ...string) {

	for _, dst := range to {
		cli, ok := s.clis[dst]
		if !ok {
			l.Warnf("cli ID %s not found", dst)
			return
		}

		// send data to ws client
		if err := wsutil.WriteServerText(cli.conn, msg); err != nil {
			l.Errorf("wsutil.WriteServerText(): %s", err.Error())
			return
		}
	}
}

func (s *Server) AddCli(c *Cli) {
	s.wscliCh <- c
}

func (s *Server) Heartbeat(id string) {
	if s.hbinterval > 0 {
		s.hbCh <- id
	} else {
		l.Warn("max heartbeat interval not set")
	}
}

func (s *Server) SendServerMsg(msg []byte, to ...string) {
	s.sendMsgCh <- &srvmsg{
		to:  to,
		msg: msg,
	}
}