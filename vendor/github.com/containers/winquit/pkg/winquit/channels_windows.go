package winquit

import (
	"os"
	"syscall"
)

type baseChannelType interface {
	getKey() any
	notifyNonBlocking()
	notifyBlocking()
}

type boolChannelType struct {
	channel chan bool
}

func (b *boolChannelType) getKey() any {
	return b.channel
}

func (b *boolChannelType) notifyNonBlocking() {
	select {
	case b.channel <- true:
	default:
	}
}

func (s *boolChannelType) notifyBlocking() {
	s.channel <- true
}

type sigChannelType struct {
	channel chan os.Signal
}

func (s *sigChannelType) getKey() any {
	return s.channel
}

func (s *sigChannelType) notifyNonBlocking() {
	select {
	case s.channel <- syscall.SIGTERM:
	default:
	}
}

func (s *sigChannelType) notifyBlocking() {
	s.channel <- syscall.SIGTERM
}
