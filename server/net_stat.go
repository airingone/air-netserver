package server

import "sync/atomic"

//计数网络包

var RecvBytesUdp uint64
var RecvPkgsUdp uint64
var SendBytesUdp uint64
var SendPkgsUdp uint64

var RecvBytesTcp uint64
var RecvPkgsTcp uint64
var SendBytesTcp uint64
var SendPkgsTcp uint64

func AddUdpStatRecv(byteNum uint64, pkgNum uint64) {
	atomic.AddUint64(&RecvBytesUdp, byteNum)
	atomic.AddUint64(&RecvPkgsUdp, pkgNum)
}

func AddUdpStatSend(byteNum uint64, pkgNum uint64) {
	atomic.AddUint64(&SendBytesUdp, byteNum)
	atomic.AddUint64(&SendPkgsUdp, pkgNum)
}

func AddTcpStatRecv(byteNum uint64, pkgNum uint64) {
	atomic.AddUint64(&RecvBytesTcp, byteNum)
	atomic.AddUint64(&RecvPkgsTcp, pkgNum)
}

func AddTcpStatSend(byteNum uint64, pkgNum uint64) {
	atomic.AddUint64(&SendBytesTcp, byteNum)
	atomic.AddUint64(&SendPkgsTcp, pkgNum)
}

func GetUdpStat() (uint64, uint64, uint64, uint64) {
	return RecvBytesUdp, RecvPkgsUdp, SendBytesUdp, SendPkgsUdp
}

func GetTcpStat() (uint64, uint64, uint64, uint64) {
	return RecvBytesTcp, RecvPkgsTcp, SendBytesTcp, SendPkgsTcp
}
