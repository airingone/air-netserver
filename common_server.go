package air_netserver

import (
	"bytes"
	"context"
	"encoding/binary"
	"errors"
	"github.com/airingone/air-netserver/server"
	"github.com/airingone/config"
	"sync"
	"time"
)

//协议server handler实现
//协议server需要实现NetChecker，NetHandler，NetLimiter（可以选择server.net_limiter.go中已实现的）
//common协议定义："(packetsize,cmdsize,subcmdsize,cmd,subcmd,data)","(uint32,uint32,uint32,string,string,string)"

//业务处理handler
type CommonFuncHandler struct {
	handler map[string]server.FuncHandler
	rmu     sync.RWMutex
}

var commonFuncHandler CommonFuncHandler

//注册业务处理函数
func RegisterCommonFuncHandler(cmd string, subCmd string, handler server.FuncHandler) {
	if commonFuncHandler.handler == nil {
		commonFuncHandler.handler = make(map[string]server.FuncHandler)
	}
	commonFuncHandler.rmu.Lock()
	defer commonFuncHandler.rmu.Unlock()
	commonFuncHandler.handler[cmd+"/"+subCmd] = handler
}

//get业务处理函数
func GetCommonFuncHandler(cmd string, subCmd string) (server.FuncHandler, error) {
	if commonFuncHandler.handler == nil {
		return nil, errors.New("not init")
	}

	commonFuncHandler.rmu.RLock()
	defer commonFuncHandler.rmu.Unlock()
	if _, ok := commonFuncHandler.handler[cmd+"/"+subCmd]; !ok {
		return nil, errors.New("not support")
	}
	return commonFuncHandler.handler[cmd+"/"+subCmd], nil
}

const (
	PacketStart uint8 = 0x28
	PacketEnd   uint8 = 0x29
)

//common checker实现
type CommonChecker struct {
}

//common check
func (c *CommonChecker) Check(data []byte) (int, error) {
	dataLen := len(data)
	if dataLen < 14 {
		return -1, errors.New("packet err")
	}
	if data[0] != PacketStart || data[dataLen-1] != PacketEnd {
		return -1, errors.New("packet format err")
	}
	/*
		sizeBuf := bytes.NewBuffer(data[1:13])
		var pkgLen, cmdLen, subCmdLen uint32
		if err := binary.Read(sizeBuf, binary.BigEndian, &pkgLen); err != nil {
			return -1, errors.New("packet read pkgLen err")
		}
		if err := binary.Read(sizeBuf, binary.BigEndian, &cmdLen); err != nil {
			return -1, errors.New("packet read pkgLen err")
		}
		if err := binary.Read(sizeBuf, binary.BigEndian, &subCmdLen); err != nil {
			return -1, errors.New("packet read pkgLen err")
		}
		if uint32(dataLen) != 1 + 12 + pkgLen + cmdLen + subCmdLen + 1 {
			return -1, errors.New("packet read pkgLen err")
		}
	*/
	return dataLen, nil
}

func NewCommonChecker() *CommonChecker {
	return &CommonChecker{}
}

//common handler
type CommonHandler struct {
}

//handler
func (h *CommonHandler) Server(ctx context.Context, data []byte) ([]byte, error) {
	//解包
	cmd, subCmd, reqBody, err := CommonHandlerUnPacket(data)
	if err != nil {
		return []byte("request packet err"), errors.New("request packet err")
	}
	//handle处理
	handle, err := GetCommonFuncHandler(cmd, subCmd)
	if err != nil {
		return []byte("cmd not support"), errors.New("cmd not support")
	}

	rspBody, err := handle(ctx, reqBody)

	//封装网络包
	rspData, err := CommonHandlerPacket([]byte(cmd), []byte(subCmd), rspBody)
	if err != nil {
		return []byte("packet rsp data err"), errors.New("packet rsp data err")
	}

	return rspData, nil
}

//parse packet data
func CommonHandlerUnPacket(data []byte) (string, string, []byte, error) {
	dataLen := len(data)
	if dataLen < 14 {
		return "", "", nil, errors.New("packet err")
	}
	if data[0] != PacketStart || data[dataLen-1] != PacketEnd {
		return "", "", nil, errors.New("packet format err")
	}

	sizeBuf := bytes.NewBuffer(data[1:13])
	var pkgLen, cmdLen, subCmdLen uint32
	if err := binary.Read(sizeBuf, binary.BigEndian, &pkgLen); err != nil {
		return "", "", nil, errors.New("packet read pkgLen err")
	}
	if err := binary.Read(sizeBuf, binary.BigEndian, &cmdLen); err != nil {
		return "", "", nil, errors.New("packet read pkgLen err")
	}
	if err := binary.Read(sizeBuf, binary.BigEndian, &subCmdLen); err != nil {
		return "", "", nil, errors.New("packet read pkgLen err")
	}

	if uint32(dataLen) != 1+12+pkgLen+cmdLen+subCmdLen+1 {
		return "", "", nil, errors.New("packet read pkgLen err")
	}

	var readLen uint32
	readLen = 1 + 3*4
	cmdBytes := data[readLen : readLen+cmdLen]
	readLen += cmdLen
	subBytes := data[readLen : readLen+subCmdLen]
	readLen += subCmdLen
	bodyBytes := data[readLen : dataLen-1]

	return string(cmdBytes), string(subBytes), bodyBytes, nil
}

//packet data
func CommonHandlerPacket(cmd []byte, subcmd []byte, data []byte) ([]byte, error) {
	cmdLen := uint32(len(cmd))
	subcmdLen := uint32(len(subcmd))
	dataLen := uint32(len(data))

	buf := new(bytes.Buffer)
	packetLen := 1 + 12 + cmdLen + subcmdLen + dataLen + 1
	buf.Grow(int(packetLen))

	if err := binary.Write(buf, binary.BigEndian, PacketStart); err != nil {
		return nil, err
	}
	if err := binary.Write(buf, binary.BigEndian, cmdLen); err != nil {
		return nil, err
	}
	if err := binary.Write(buf, binary.BigEndian, subcmdLen); err != nil {
		return nil, err
	}
	if err := binary.Write(buf, binary.BigEndian, dataLen); err != nil {
		return nil, err
	}
	if cmdLen > 0 {
		if err := binary.Write(buf, binary.BigEndian, cmd); err != nil {
			return nil, err
		}
	}
	if subcmdLen > 0 {
		if err := binary.Write(buf, binary.BigEndian, subcmd); err != nil {
			return nil, err
		}
	}
	if dataLen > 0 {
		if err := binary.Write(buf, binary.BigEndian, data); err != nil {
			return nil, err
		}
	}
	if err := binary.Write(buf, binary.BigEndian, PacketEnd); err != nil {
		return nil, err
	}

	return buf.Bytes(), nil
}

func NewCommonHandler() *CommonHandler {
	return &CommonHandler{}
}

//启动common udp服务
func CommonListenAndServeUdp(addr string, config config.ConfigServer) {
	check := NewCommonChecker()
	limit := server.NewTokenBucketLimiter(int64(config.CapacityLimit))
	handler := NewCommonHandler()
	server.ListenAndServeUdp(addr, check, limit, handler,
		time.Duration(config.NetTimeOutMs), uint64(config.CapacityPool))
}
