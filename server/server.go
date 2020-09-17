package server

import "context"

const (
	ClientAddrKey = "clientAddr"
)

//网络包checker
//input:数据包
//return: 0,nil-not finish, >0,nil:finish且返回长度 <0,err: error
type NetChecker interface {
	Check([]byte) (int, error)
}

//网络处理handler
type NetHandler interface {
	Server(context.Context, []byte) ([]byte, error)
}

//限频接口
type NetLimiter interface {
	Acquire() bool
}

//业务处理函数
type FuncHandler func(context.Context, []byte) ([]byte, error)
