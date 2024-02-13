package main

import (
	"compress/flate"
	"crypto/tls"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"net"
	"sync"

	"github.com/kelseyhightower/envconfig"
)

type compressionWriter interface {
	io.WriteCloser
	Flush() error
}

var config Config

func main() {
	envconfig.MustProcess("", &config)

	slog.Info(fmt.Sprintf("Config: %+v", config))

	ln, err := net.Listen("tcp", "0.0.0.0:"+config.Port)
	if err != nil {
		panic(err)
	}
	for {
		c, err := ln.Accept()
		if err != nil {
			slog.Error(fmt.Sprintf("accept: %v", err))
			continue
		}
		go serve(c)
	}
}

func serve(c net.Conn) {
	binanceConn, err := tls.Dial("tcp", config.BinanceHostPort, &tls.Config{InsecureSkipVerify: config.InsecureSkipVerify})
	if err != nil {
		slog.Error(fmt.Sprintf("dial Binance: %v", err))
		c.Close()
		return
	}
	slog.Info("connected to Binance")

	var wg sync.WaitGroup
	wg.Add(2)

	go func() {
		defer wg.Done()
		if _, err := io.Copy(binanceConn, c); err != nil {
			slog.Error(fmt.Sprintf("copy client to Binance: %v", err))
		}
		if err := binanceConn.Close(); err != nil {
			slog.Warn(fmt.Sprintf("close Binance connection: %v", err))
		}
	}()

	go func() {
		defer wg.Done()
		defer func() {
			if err := c.Close(); err != nil {
				slog.Warn(fmt.Sprintf("close client connection: %v", err))
			}
		}()
		w, err := flate.NewWriter(c, config.CompressionLevel)
		if err != nil {
			slog.Error(fmt.Sprintf("new writer for c: %v", err))
			return
		}
		defer func() {
			if err := w.Close(); err != nil {
				slog.Error(fmt.Sprintf("close writer: %v", err))
			}
		}()
		if _, err := copyWithFlush(w, binanceConn); err != nil {
			slog.Error(fmt.Sprintf("copy Binance to client: %v", err))
		}
	}()

	wg.Wait()
}

// Copied from io.Copy and modified
func copyWithFlush(dst compressionWriter, src io.Reader) (written int64, err error) {
	size := 32 * 1024
	if l, ok := src.(*io.LimitedReader); ok && int64(size) > l.N {
		if l.N < 1 {
			size = 1
		} else {
			size = int(l.N)
		}
	}
	buf := make([]byte, size)

	for {
		nr, er := src.Read(buf)
		if nr > 0 {
			nw, ew := dst.Write(buf[0:nr])
			if nw < 0 || nr < nw {
				nw = 0
				if ew == nil {
					ew = errors.New("invalid write result")
				}
			}
			written += int64(nw)
			if ew != nil {
				err = ew
				break
			}
			if nr != nw {
				err = errors.New("short write")
				break
			}
			err = dst.Flush()
			if err != nil {
				break
			}
		}
		if er != nil {
			if er != io.EOF {
				err = er
			}
			break
		}
	}
	return written, err
}
