package service

import (
	"io"
	"net"
	"sync"
	"sync/atomic"
)

const (
	StateAccepted = int(iota)
	StateRunning
	StateClosed
)

var _dataporterindex = int32(0)

type DataPorter struct {
	Index        int32
	hubIndex     int
	State        int
	SvcName      string
	Src          net.Conn
	SrcAddr      string
	Dist         net.Conn
	DistAddr     string
	UploadSize   int32
	DownloadSize int32
	CloseErr     error
}

func NewDataPorter(name string, src net.Conn) *DataPorter {
	return &DataPorter{
		Index:   atomic.AddInt32(&_dataporterindex, 1),
		SvcName: name,
		Src:     src,
		SrcAddr: src.RemoteAddr().String(),
		State:   StateAccepted,
	}
}

func (r *DataPorter) SetHubIndex(index int) {
	r.hubIndex = index
}
func (r *DataPorter) GetHubIndex() int {
	return r.hubIndex
}

func (r *DataPorter) LinkDist(dist net.Conn) {
	r.Dist = dist
	r.DistAddr = dist.RemoteAddr().String()
	r.State = StateRunning

	var mut sync.Mutex
	setErr := func(err error) {
		mut.Lock()
		defer mut.Unlock()
		if r.CloseErr != nil {
			return
		}
		r.CloseErr = err
	}

	var wg sync.WaitGroup
	wg.Add(2)
	go copy(&wg, r.Dist, r.Src, &r.UploadSize, setErr)
	go copy(&wg, r.Src, r.Dist, &r.DownloadSize, setErr)
	wg.Wait()

	r.State = StateClosed
	r.Src = nil
	r.Dist = nil
}

func copy(wg *sync.WaitGroup, dist io.WriteCloser, src io.ReadCloser, count *int32, setErr func(error)) {
	defer func() {
		src.Close()
		dist.Close()
		wg.Done()
	}()

	buf := make([]byte, 1024*64)
	for {
		rn, readErr := src.Read(buf)
		if rn > 0 {
			wn, writeErr := dist.Write(buf[:rn])
			*count += int32(wn)
			if writeErr != nil {
				setErr(writeErr)
				return
			}
		}
		if readErr != nil {
			setErr(readErr)
			return
		}
	}
}

type DataPorterHub struct {
	svcName string
	actives []*DataPorter
	dones   []*DataPorter
	mut     sync.Mutex
}

func NewDataPorterHub(svcName string) *DataPorterHub {
	return &DataPorterHub{
		svcName: svcName,
		actives: make([]*DataPorter, 0),
		dones:   make([]*DataPorter, 0),
	}
}

func (hub *DataPorterHub) NewPorter(c1 net.Conn) *DataPorter {
	porter := NewDataPorter(hub.svcName, c1)

	hub.mut.Lock()
	hub.actives = append(hub.actives, porter)
	porter.SetHubIndex(len(hub.actives) - 1)
	hub.mut.Unlock()

	return porter
}

func (hub *DataPorterHub) DonePorter(porter *DataPorter) {
	hub.mut.Lock()

	index := porter.GetHubIndex()
	sz := len(hub.actives)
	if index >= 0 && index < sz {
		hub.actives[index] = hub.actives[sz-1]
		hub.actives = hub.actives[:sz-1]
		hub.actives[index].SetHubIndex(index)

		hub.dones = append(hub.dones, porter)
	}

	hub.mut.Unlock()
}

func (hub *DataPorterHub) NumActives() int32 {
	return int32(len(hub.actives))
}

func (hub *DataPorterHub) NumDones() int32 {
	return int32(len(hub.dones))
}
