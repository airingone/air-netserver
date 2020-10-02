package server

import "sync/atomic"
//计数网络包

//udp包统计
var RecvBytesUdp uint64
var RecvPkgsUdp uint64
var SendBytesUdp uint64
var SendPkgsUdp uint64

//tcp包统计
var RecvBytesTcp uint64
var RecvPkgsTcp uint64
var SendBytesTcp uint64
var SendPkgsTcp uint64

//udp接受包统计
//byteNum: 包大小
//pkgNum: 包数
func AddUdpStatRecv(byteNum uint64, pkgNum uint64) {
	atomic.AddUint64(&RecvBytesUdp, byteNum)
	atomic.AddUint64(&RecvPkgsUdp, pkgNum)
}

//udp发送包统计
//byteNum: 包大小
//pkgNum: 包数
func AddUdpStatSend(byteNum uint64, pkgNum uint64) {
	atomic.AddUint64(&SendBytesUdp, byteNum)
	atomic.AddUint64(&SendPkgsUdp, pkgNum)
}

//tcp接受包统计
//byteNum: 包大小
//pkgNum: 包数
func AddTcpStatRecv(byteNum uint64, pkgNum uint64) {
	atomic.AddUint64(&RecvBytesTcp, byteNum)
	atomic.AddUint64(&RecvPkgsTcp, pkgNum)
}

//udp发送包统计
//byteNum: 包大小
//pkgNum: 包数
func AddTcpStatSend(byteNum uint64, pkgNum uint64) {
	atomic.AddUint64(&SendBytesTcp, byteNum)
	atomic.AddUint64(&SendPkgsTcp, pkgNum)
}

//读取udp统计
func GetUdpStat() (uint64, uint64, uint64, uint64) {
	return RecvBytesUdp, RecvPkgsUdp, SendBytesUdp, SendPkgsUdp
}

//读取tcp统计
func GetTcpStat() (uint64, uint64, uint64, uint64) {
	return RecvBytesTcp, RecvPkgsTcp, SendBytesTcp, SendPkgsTcp
}
