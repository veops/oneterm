package ssh

import (
	"net"
	"time"

	"github.com/pires/go-proxyproto"

	"github.com/veops/oneterm/pkg/logger"
	"github.com/veops/oneterm/pkg/proto/ssh/handler"
)

func Run(Addr, apiHost, token, privateKeyPath, secretKey string) error {
	s, er := handler.Init(Addr, apiHost, token, privateKeyPath, secretKey)

	if er != nil {
		return er
	}
	go func() {
		ln, err := net.Listen("tcp", s.Addr)
		if err != nil {
			logger.L.Fatal(err.Error())
		}

		proxyListener := &proxyproto.Listener{
			Listener:          ln,
			ReadHeaderTimeout: 10 * time.Second,
		}
		defer proxyListener.Close()

		err = s.Serve(proxyListener)
		if err != nil {
			logger.L.Fatal(err.Error())
			return
		}

	}()

	return nil
}
