package server

import (
	"context"
	"errors"
	"github.com/airingone/log"
	"net"
	"sync"
	"time"
)

const (
	TcpKeepAlive = "keepalive"
)

var TcpKeepAliveLen = len([]byte("keepalive"))

func init() {
	TcpRecvBufPool = &sync.Pool{
		New: func() interface{} {
			return make([]byte, TcpMaxRecvbuf)
		},
	}
}

//psuh chan的数据结构
type TcpPushData struct {
	addr string
	data []byte
}

//用于push功能
type TcpPush struct {
	server  *TcpServer
	cPush   chan TcpPushData      //push消息通道
	mClient map[string]*TcpClient //缓冲连接
	rmu     sync.RWMutex
}

var tcpPush *TcpPush

//连接保存在session
func TcpPushAddClient(addr string, client *TcpClient) error {
	if tcpPush == nil {
		return errors.New("tcp push not init")
	}
	tcpPush.rmu.Lock()
	defer tcpPush.rmu.Unlock()
	tcpPush.mClient[addr] = client

	return nil
}

//连接从session删除
func TcpPushDeleteClient(addr string, client *TcpClient) error {
	if tcpPush == nil {
		return errors.New("tcp push not init")
	}
	tcpPush.rmu.Lock()
	defer tcpPush.rmu.Unlock()
	delete(tcpPush.mClient, addr)

	return nil
}

//从session中获取连接并发送请求
func (p *TcpPush) writeClient(data TcpPushData) error {
	p.rmu.RLock()
	defer p.rmu.RUnlock()
	if _, ok := p.mClient[data.addr]; !ok {
		return errors.New("tcp connect not exist")
	}

	//将数据发送到client write chan
	select {
	case p.mClient[data.addr].cout <- data.data:
		break
	}

	return nil
}

//初始化push
func InitTcpPush(server *TcpServer) {
	push := &TcpPush{
		server:  server,
		cPush:   make(chan TcpPushData, 1000),
		mClient: make(map[string]*TcpClient),
		rmu:     sync.RWMutex{},
	}
	tcpPush = push

	go func() {
		defer func() {
			if r := recover(); r != nil {
				log.PanicTrack()
			}

			for !tcpPush.server.stoping {
				select { //接受PushDataToClient的psuh
				case pushData := <-tcpPush.cPush:
					tcpPush.writeClient(pushData)
				}
			}
		}()
	}()
}

//发起push，业务逻辑调用
func PushDataToClient(addr string, data []byte) error {
	if tcpPush == nil {
		return errors.New("tcp push not init")
	}
	pushData := TcpPushData{
		addr: addr,
		data: data,
	}

	timeout := time.NewTimer(200 * time.Millisecond)
	select {
	case tcpPush.cPush <- pushData:
		break
	case <-timeout.C:
		return errors.New("send to chan timeout")
	}

	return nil
}

//用于recv buf的缓冲分配，避免频繁分配与释放,协程安全
var TcpRecvBufPool *sync.Pool

const (
	TcpMaxRecvbuf = 1024 * 128 //128k
)

//listen收到请求后建立的client
type TcpClient struct {
	server    *TcpServer
	conn      net.Conn //client conn
	peerAddr  string   //clinet addr
	localAddr *net.TCPAddr
	cancelCtx context.CancelFunc
	cin       chan []byte //read data chan
	cout      chan []byte //write data chan
	wg        sync.WaitGroup
}

//处理tcp连接
func (c *TcpClient) Handle() error {
	c.peerAddr = c.conn.RemoteAddr().String()
	ctx := context.WithValue(context.Background(), ServerAddrKey, c.localAddr.String())
	ctx = context.WithValue(ctx, ClientAddrKey, c.peerAddr)
	ctx, cancle := context.WithCancel(ctx)
	c.cancelCtx = cancle
	defer c.conn.Close()

	if c.server.isPush { //add client to push
		TcpPushAddClient(c.peerAddr, c)
	}

	c.wg.Add(3)
	go c.readPacket(ctx)
	go c.handleWorker(ctx)
	go c.writePacket(ctx)
	c.wg.Wait()

	if c.server.isPush { //delete client to push
		TcpPushDeleteClient(c.peerAddr, c)
	}

	log.Info("[NETSERVER]: close connect, client addr: %s", c.peerAddr)

	return nil
}

//tcp读网络数据包
func (c *TcpClient) readPacket(ctx context.Context) {
	defer func() {
		if r := recover(); r != nil {
			log.PanicTrack()
		}
	}()
	defer c.cancelCtx()
	defer c.wg.Done()

	recvBuf := TcpRecvBufPool.Get().([]byte)
	defer TcpRecvBufPool.Put(recvBuf)

	var nRead int
	for !c.server.stoping {
		//_ = c.conn.SetReadDeadline(time.Now().Add(c.server.idleTimeout))
		_ = c.conn.SetReadDeadline(time.Now().Add(300 * time.Second))
		num, err := c.conn.Read(recvBuf[nRead:])
		if num >= TcpKeepAliveLen && string(recvBuf[0:TcpKeepAliveLen]) == TcpKeepAlive {
			select {
			case <-ctx.Done():
				return
			case c.cout <- []byte("keepalive"):
				log.Info("[NETSERVER]: keepalive")
			}

			if num > TcpKeepAliveLen {
				num -= TcpKeepAliveLen
				copy(recvBuf, recvBuf[TcpKeepAliveLen:])
			} else {
				continue
			}
		}
		//限频处理
		if !c.server.limiter.Acquire() {
			log.Info("[NETSERVER]: ListenAndServeTcp limiter.Acquire() is true, client addr: %s", c.peerAddr)
			continue
		}
		AddTcpStatRecv(uint64(num), 1)
		if err != nil { //表示idleTimeout时间内无数据请求，则关闭连接
			if c.server.stoping {
				break
			}
			log.Info("[NETSERVER]: tcp conn close, client addr: %s, err: %+v", c.peerAddr, err)
			return
		}
		nRead += num

		if nRead >= cap(recvBuf) { //buffer满了
			tmpRecvBuf := make([]byte, nRead*2)
			copy(tmpRecvBuf, recvBuf[:nRead])
			recvBuf = tmpRecvBuf
		}

		var index int
		for {
			pkgLen, err := c.server.checker.Check(recvBuf[index:nRead])
			if err != nil || pkgLen < 0 {
				log.Info("[NETSERVER]: checker.Check err, client addr: %s, err: %+v, data: %s", c.peerAddr, err, string(recvBuf[index:nRead]))
				return //数据格式错误或超长，关闭当前连接，需client重新建立连接
			}

			if pkgLen == 0 { //未接受完
				break
			}

			//已接受完成
			reqBytes := make([]byte, pkgLen)
			copy(reqBytes, recvBuf[index:index+pkgLen])
			select {
			case <-ctx.Done():
				return
			case c.cin <- reqBytes:
				index += pkgLen
			}
			if index >= nRead {
				break
			}
		}

		if index > 0 {
			if index < nRead { //剩下的第二个包未接受完的情况
				copy(recvBuf, recvBuf[index:nRead])
			}
			nRead -= index
		}
	}

	if c.server.stoping {
		time.Sleep(time.Second * 5)
	}
}

//tcp handle业务处理
func (c *TcpClient) handleWorker(ctx context.Context) {
	defer func() {
		if r := recover(); r != nil {
			c.conn.SetReadDeadline(time.Now())
			log.PanicTrack()
		}
	}()
	defer c.cancelCtx()
	defer c.wg.Done()

	for {
		select {
		case <-ctx.Done():
			return
		case req := <-c.cin:
			go func() {
				defer func() {
					if r := recover(); r != nil {
						log.PanicTrack()
					}
				}()

				subCtx, cancel := context.WithTimeout(ctx, c.server.timeoutMs)
				rsp, err := c.server.handler.Server(subCtx, req)
				cancel()
				if err != nil {
					return
				}
				select {
				case <-ctx.Done():
					return
				case c.cout <- rsp: //将返回数据发送给回包协程

				}
			}()
		}
	}
}

//tcp写网络数据包
func (c *TcpClient) writePacket(ctx context.Context) {
	defer func() {
		if r := recover(); r != nil {
			log.PanicTrack()
		}
	}()
	defer c.cancelCtx()
	defer c.wg.Done()

	for {
		select {
		case <-ctx.Done():
			return
		case rsp := <-c.cout:
			_ = c.conn.SetWriteDeadline(time.Now().Add(c.server.idleTimeout))
			num, err := c.conn.Write(rsp)
			if err != nil {
				return
			}
			AddTcpStatSend(uint64(num), 1)
		}
	}
}

//tcp server
type TcpServer struct {
	checker     NetChecker  //数据包check函数
	limiter     NetLimiter  //限流函数
	handler     NetHandler  //业务处理函数
	workerPoll  *WorkerPool //工作协程池
	addr        *net.TCPAddr
	listen      *net.TCPListener
	timeoutMs   time.Duration
	idleTimeout time.Duration //长连接维持时间，比如5min没请求则关闭连接
	stoping     bool
	isPush      bool //是否开启push
}

func (s *TcpServer) ListenAndServe() error {
	//listen tcp
	conn, err := net.ListenTCP("tcp", s.addr)
	if err != nil {
		log.Fatal("[NETSERVER]: ListenAndServe ListenTCP err, err: %+v", err)
	}
	s.listen = conn
	defer s.listen.Close()

	if s.isPush { //开启push协程
		InitTcpPush(s)
	}

	var netDelay time.Duration
	for !s.stoping {
		cConn, err := s.listen.Accept()
		if err != nil { //recv err
			if s.stoping {
				break
			}
			if ne, ok := err.(net.Error); ok && ne.Temporary() { //Temporary err
				if netDelay == 0 {
					netDelay = 5 * time.Millisecond
				} else {
					netDelay *= 2
				}
				if netDelay > 1*time.Second {
					netDelay = 1 * time.Second
				}
				time.Sleep(netDelay) //错误时delay
				continue
			}
			log.Info("[NETSERVER]: ListenAndServeTcp listen.Accept err, err: %+v", err)
			break //其他错误则退出
		}
		netDelay = 0

		//创建接受client
		client := &TcpClient{
			server:    s,
			conn:      cConn,
			localAddr: s.addr,
			cin:       make(chan []byte, 20),
			cout:      make(chan []byte, 21),
		}
		err = s.workerPoll.Put(client) //使用协程池
		//go client.Handle()
		if err != nil {
			_, _ = client.conn.Write([]byte("server worker rate limit"))
			_ = client.conn.Close()
		}
	}

	log.Error("[NETSERVER]: tcp server close")

	return nil
}

//shutdown
func (s *TcpServer) Shutdown() error {
	if !s.stoping {
		s.stoping = true
	}

	return nil
}

//为tcp统一的Listen接口，根据自身协议需要需要实现checker，limiter，handler
func ListenAndServeTcp(addr string, checker NetChecker, limiter NetLimiter, handler NetHandler,
	timeoutMs time.Duration, idleTimeout time.Duration, capacity uint64, isPush bool) {
	netAddr, err := net.ResolveTCPAddr("tcp", addr)
	if err != nil {
		return
	}
	s := &TcpServer{
		checker:     checker,
		limiter:     limiter,
		handler:     handler,
		workerPoll:  NewWorkPool(capacity),
		timeoutMs:   timeoutMs,
		stoping:     false,
		addr:        netAddr,
		idleTimeout: idleTimeout,
		isPush:      isPush,
	}

	_ = s.ListenAndServe()
}
