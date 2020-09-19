package server

import (
	"context"
	"github.com/airingone/log"
	"net"
	"time"
)

const (
	UdpMaxRecvbuf = 1024 * 128 //128k
)

//listen收到请求后建立的client
type UdpClient struct {
	server    *UdpServer
	reqData   []byte
	conn      *net.UDPConn
	localAddr *net.UDPAddr
	peerAddr  *net.UDPAddr
	//handler   NetHandler
	timeoutMs time.Duration
}

//client handler
func (c *UdpClient) Handle() error {
	ctx, canel := context.WithTimeout(context.Background(), c.timeoutMs)
	defer canel()
	ctx = context.WithValue(ctx, ClientAddrKey, c.peerAddr.String())
	rspData, err := c.server.handler.Server(ctx, c.reqData) //业务处理
	if err == nil && len(rspData) > 0 {
		if len(rspData) > UdpMaxRecvbuf {
			log.Info("[NETSERVER]: Handle udp send packet too max, %d", len(rspData))
		}

		if sendNum, err := c.conn.WriteToUDP(rspData, c.peerAddr); err == nil {
			AddUdpStatSend(uint64(sendNum), 1)
		} else {
			log.Info("[NETSERVER]: Handle udp send packet err")
		}
	} else {
		log.Error("[NETSERVER]: Handle udp process err, %+v", err)
	}
	log.Info("[NETSERVER]: succ, stat:recvbyte=%d, recvcnt=%d, sendbytes=%d, sendcnt%d", RecvBytesUdp, RecvPkgsUdp, SendBytesUdp, SendPkgsUdp)

	return nil
}

//server
type UdpServer struct {
	checker    NetChecker  //数据包check函数
	limiter    NetLimiter  //限流函数
	handler    NetHandler  //业务处理函数
	workerPoll *WorkerPool //工作协程池
	addr       *net.UDPAddr
	conn       *net.UDPConn
	timeoutMs  time.Duration
	stoping    bool
}

func (s *UdpServer) ListenAndServe() error {
	//listen udp
	conn, err := net.ListenUDP("udp", s.addr)
	if err != nil {
		log.Fatal("[NETSERVER]: ListenAndServe ListenUDP err, err: %+v", err)
	}
	s.conn = conn
	defer s.conn.Close()

	var netDelay time.Duration
	rBuf := make([]byte, UdpMaxRecvbuf)
	for !s.stoping {
		rNum, rAddr, err := s.conn.ReadFromUDP(rBuf)
		AddUdpStatRecv(uint64(rNum), 1) //统计网络接受数
		if err != nil {                 //recv err
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
			log.Info("[NETSERVER]: ListenAndServeUdp ReadFromUDP err, err: %+v", err)
			break //其他错误则退出
		}
		netDelay = 0

		//限频处理
		if !s.limiter.Acquire() {
			log.Info("[NETSERVER]: ListenAndServeUdp limiter.Acquire() is true")
			continue
		}

		//check数据包
		if num, err := s.checker.Check(rBuf[:rNum]); num <= 0 || err != nil { //udp要求一个请求是完整的请求包
			log.Info("[NETSERVER]: ListenAndServeUdp checker.Check err")
			continue
		}

		//创建src client
		rClient := &UdpClient{
			server:    s,
			reqData:   make([]byte, rNum, rNum),
			localAddr: s.addr,
			peerAddr:  rAddr,
			conn:      s.conn,
			timeoutMs: s.timeoutMs,
		}
		copy(rClient.reqData, rBuf[0:rNum])
		err = s.workerPoll.Put(rClient) //使用协程池
		//go rClient.handler.Server(context.Background(), rClient.reqData)
		if err != nil {
			_, _ = rClient.conn.WriteToUDP([]byte("server worker rate limit"), rAddr)
		}
	}
	log.Error("[NETSERVER]: tcp server close")

	return nil
}

//shutdown
func (s *UdpServer) Shutdown() error {
	if !s.stoping {
		s.stoping = true
		s.conn.SetReadDeadline(time.Now())
	}

	return nil
}

//为udp统一的Listen接口，根据自身协议需要需要实现checker，limiter，handler
func ListenAndServeUdp(addr string, checker NetChecker, limiter NetLimiter, handler NetHandler,
	timeoutMs time.Duration, capacity uint64) {
	listenAddr, err := net.ResolveUDPAddr("udp4", addr)
	if err != nil {
		log.Fatal("[NETSERVER]: ListenAndServeUdp ResolveUDPAddr err, err: %+v", err)
	}
	s := &UdpServer{
		checker:    checker,
		limiter:    limiter,
		handler:    handler,
		workerPoll: NewWorkPool(capacity),
		addr:       listenAddr,
		timeoutMs:  timeoutMs,
		stoping:    false,
	}

	_ = s.ListenAndServe()
}
