package server

import (
	"errors"
	"github.com/airingone/log"
	"sync"
	"sync/atomic"
	"time"
)

const (
	WorkerPoolStateRun        = 1
	WorkerPoolStateStop       = 2
	WorkerPoolDefaultCapacity = 10000
)

//业务处理任务接口，如UdpClient,需要实现Handle
type WorkerHandler interface {
	Handle() error
}

type WorkerTask struct {
	Handler WorkerHandler
}

//协程池
type WorkerPool struct {
	capacity      uint64           //协程池容量，初始化时设置
	runingWorkers uint64           //当前协程工作数
	state         uint32           //协程池工作状态
	tackC         chan *WorkerTask //任务队列
	mutex         sync.Mutex
}

//创建
func NewWorkPool(capacity uint64) *WorkerPool {
	if capacity < 0 {
		capacity = WorkerPoolDefaultCapacity
	}

	pool := &WorkerPool{
		capacity:      capacity,
		runingWorkers: 0,
		state:         WorkerPoolStateRun,
		tackC:         make(chan *WorkerTask, capacity),
	}

	return pool
}

//启动一个协程处理任务
func (p *WorkerPool) run() {
	atomic.AddUint64(&p.runingWorkers, 1)

	go func() { //启动协程等待执行任务
		defer atomic.AddUint64(&p.runingWorkers, ^uint64(0))
		defer func() {
			if r := recover(); r != nil {
				log.PanicTrack()
			}
		}()

		for {
			select { //循环等待chan数据
			case task, ok := <-p.tackC:
				if !ok { //channel被关闭
					return
				}
				task.Handler.Handle()
			}
		}
	}()
}

//add任务
func (p *WorkerPool) Put(handle WorkerHandler) error {
	if p.state != WorkerPoolStateRun {
		return errors.New("WorkerPool is stop")
	}

	p.mutex.Lock()
	if p.runingWorkers < p.capacity {
		p.run()
	}
	p.mutex.Unlock()

	timeout := time.NewTimer(200 * time.Millisecond)
	select {
	case p.tackC <- &WorkerTask{Handler: handle}:
		return nil
	case <-timeout.C: //如果chan满了会失败
		return errors.New("send chan timeout")
	}
}

//close
func (p *WorkerPool) Close() {
	if p.state == WorkerPoolStateStop {
		return
	}
	p.state = WorkerPoolStateStop
	for len(p.tackC) > 0 {
		time.Sleep(1 * time.Second)
	}

	p.mutex.Lock()
	defer p.mutex.Unlock()
	close(p.tackC)
}

//获取当前工作协程数
func (p *WorkerPool) GetWorkerNumber() uint64 {
	p.mutex.Lock()
	num := p.runingWorkers
	defer p.mutex.Unlock()

	return num
}
