// +build linux

package poll

import (
	"syscall"
	"time"
)

type Poller struct {
	epfd   int
	event  syscall.EpollEvent
	events [32]syscall.EpollEvent
}

func New(fd int) (p *Poller, err error) {
	p = &Poller{}
	if p.epfd, err = syscall.EpollCreate1(0); err != nil {
		return nil, err
	}

	p.event.Events = syscall.EPOLLOUT
	p.event.Fd = int32(fd)
	if err = syscall.EpollCtl(p.epfd, syscall.EPOLL_CTL_ADD, fd, &p.event); err != nil {
		p.Close()
		return nil, err
	}

	return p, nil
}

func (p *Poller) Close() error {
	return syscall.Close(p.epfd)
}

func (p *Poller) WaitWrite(deadline time.Time) error {
	msec := -1
	if !deadline.IsZero() {
		d := deadline.Sub(time.Now())
		msec = int(d.Nanoseconds() / 1000000) // ms!? omg...
	}

	n, err := syscall.EpollWait(p.epfd, p.events[:], msec)
	if err != nil {
		return err
	}
	if n < 1 {
		return errTimeout
	}
	return nil
}
