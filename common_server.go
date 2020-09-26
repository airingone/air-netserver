package air_netserver

import (
	"bytes"
	"context"
	"encoding/binary"
	"errors"
	"fmt"
	"github.com/airingone/air-netserver/server"
	"github.com/airingone/config"
	"github.com/airingone/log"
	"sync"
	"time"
)

//协议server handler实现:
//1.协议接口：协议server需要实现NetChecker，NetHandler，NetLimiter（可以选择server.net_limiter.go中已实现的）
//2.提供给使用方的接口：CommonListenAndServeUdp（启动服务），RegisterCommonFuncHandler（用于注册逻辑服务），ProtocolUnPacket（解包），ProtocolPacket（生成包）

//common协议定义："(serverNamesize,cmdsize,packetsize,serverName,cmd,data)","(uint32,uint32,uint32,string,string,string)"

//业务处理handler
type CommonFuncHandler struct {
	handler map[string]server.FuncHandler
	rmu     sync.RWMutex
}

var commonFuncHandler CommonFuncHandler

//注册业务处理函数
func RegisterCommonFuncHandler(serverName string, cmd string, handler server.FuncHandler) {
	if commonFuncHandler.handler == nil {
		commonFuncHandler.handler = make(map[string]server.FuncHandler)
	}
	commonFuncHandler.rmu.Lock()
	defer commonFuncHandler.rmu.Unlock()
	commonFuncHandler.handler[serverName+"/"+cmd] = handler
}

//get业务处理函数
func GetCommonFuncHandler(serverName string, cmd string) (server.FuncHandler, error) {
	if commonFuncHandler.handler == nil {
		return nil, errors.New("not init")
	}

	commonFuncHandler.rmu.RLock()
	defer commonFuncHandler.rmu.RUnlock()
	if _, ok := commonFuncHandler.handler[serverName+"/"+cmd]; !ok {
		return nil, errors.New("not support")
	}
	return commonFuncHandler.handler[serverName+"/"+cmd], nil
}

const (
	PacketStart uint8 = 0x28
	PacketEnd   uint8 = 0x29
)

//common udp checker实现
type CommonUdpChecker struct {
}

//common check
func (c *CommonUdpChecker) Check(data []byte) (int, error) {
	dataLen := len(data)
	if dataLen < 14 {
		return -1, errors.New("packet err")
	}
	if data[0] != PacketStart || data[dataLen-1] != PacketEnd {
		return -1, errors.New("packet format err")
	}

	return dataLen, nil
}

func NewCommonUdpChecker() *CommonUdpChecker {
	return &CommonUdpChecker{}
}

//common tcp checker实现
type CommonTcpChecker struct {
}

//common check
func (c *CommonTcpChecker) Check(data []byte) (int, error) {
	dataLen := uint32(len(data))
	if dataLen < 13 {
		return -1, errors.New("packet err")
	}
	if data[0] != PacketStart {
		return -1, errors.New("start packet format err")
	}
	//buf := bytes.NewBuffer(data[1:14])
	buf := bytes.NewBuffer(data[1 : dataLen-1])
	var serverNameLen, cmdLen, bodyLen uint32
	if err := binary.Read(buf, binary.BigEndian, &serverNameLen); err != nil {
		return -1, errors.New("packet read pkgLen err")
	}
	if err := binary.Read(buf, binary.BigEndian, &cmdLen); err != nil {
		return -1, errors.New("packet read pkgLen err")
	}
	if err := binary.Read(buf, binary.BigEndian, &bodyLen); err != nil {
		return -1, errors.New("packet read pkgLen err")
	}

	pkgLen := 2 + 12 + serverNameLen + cmdLen + bodyLen
	if pkgLen > dataLen { //未读完
		return 0, nil
	}

	if data[pkgLen-1] != PacketEnd {
		return -1, errors.New("end packet format err")
	}

	return int(pkgLen), nil
}

func NewCommonTcpChecker() *CommonTcpChecker {
	return &CommonTcpChecker{}
}

//common handler
type CommonHandler struct {
}

//handler
func (h *CommonHandler) Server(ctx context.Context, data []byte) ([]byte, error) {
	log.Info("[NETSERVER]: req: %s", string(data))
	//解包
	serverName, cmd, reqBody, err := CommonProtocolUnPacket(data)
	if err != nil {
		log.Info("[NETSERVER]: request packet err, %+v", err)
		return []byte("request packet err"), nil //不返回错误则会把这msg返回client
	}

	//handle处理
	handle, err := GetCommonFuncHandler(serverName, cmd)
	if err != nil {
		return []byte("cmd not support"), nil
	}

	rspBody, err := handle(ctx, reqBody)

	//封装网络包
	rspData, err := CommonProtocolPacket([]byte(serverName), []byte(cmd), rspBody)
	if err != nil {
		return []byte("packet rsp data err"), nil
	}
	log.Info("[NETSERVER]: rsp: %s", string(rspData))

	return rspData, nil
}

//parse packet data
func CommonProtocolUnPacket(data []byte) (string, string, []byte, error) {
	dataLen := len(data)
	if dataLen < 14 {
		return "", "", nil, errors.New("packet err")
	}
	if data[0] != PacketStart || data[dataLen-1] != PacketEnd {
		return "", "", nil, errors.New("packet format err")
	}

	buf := bytes.NewBuffer(data[1 : dataLen-1])
	var serverNameLen, cmdLen, bodyLen uint32
	if err := binary.Read(buf, binary.BigEndian, &serverNameLen); err != nil {
		return "", "", nil, errors.New("packet read pkgLen err")
	}
	if err := binary.Read(buf, binary.BigEndian, &cmdLen); err != nil {
		return "", "", nil, errors.New("packet read pkgLen err")
	}
	if err := binary.Read(buf, binary.BigEndian, &bodyLen); err != nil {
		return "", "", nil, errors.New("packet read pkgLen err")
	}

	if uint32(dataLen) != 2+12+bodyLen+serverNameLen+cmdLen {
		return "", "", nil, errors.New("pkgLen err")
	}

	serverNameBytes := make([]byte, serverNameLen, serverNameLen)
	if err := binary.Read(buf, binary.BigEndian, &serverNameBytes); err != nil {
		return "", "", nil, errors.New("packet read cmd err")
	}
	cmdBytes := make([]byte, cmdLen, cmdLen)
	if err := binary.Read(buf, binary.BigEndian, &cmdBytes); err != nil {
		return "", "", nil, errors.New("packet read sub cmd err")
	}
	bodyBytes := make([]byte, bodyLen, bodyLen)
	if err := binary.Read(buf, binary.BigEndian, &bodyBytes); err != nil {
		return "", "", nil, errors.New("packet read body data err")
	}

	/*	serverNameBytes := data[13 : 13+serverNameLen]
		cmdBytes := data[13+serverNameLen : 13+serverNameLen+cmdLen]
		bodyBytes := data[13+serverNameLen+cmdLen : 13+serverNameLen+cmdLen+bodyLen]
	*/
	return string(serverNameBytes), string(cmdBytes), bodyBytes, nil
}

//packet data
func CommonProtocolPacket(serverName []byte, cmd []byte, data []byte) ([]byte, error) {
	serverNameLen := uint32(len(serverName))
	cmdLen := uint32(len(cmd))
	bodyLen := uint32(len(data))

	buf := new(bytes.Buffer)
	packetLen := 1 + 12 + serverNameLen + cmdLen + bodyLen + 1
	buf.Grow(int(packetLen))

	if err := binary.Write(buf, binary.BigEndian, PacketStart); err != nil {
		return nil, err
	}
	if err := binary.Write(buf, binary.BigEndian, serverNameLen); err != nil {
		return nil, err
	}
	if err := binary.Write(buf, binary.BigEndian, cmdLen); err != nil {
		return nil, err
	}
	if err := binary.Write(buf, binary.BigEndian, bodyLen); err != nil {
		return nil, err
	}
	if serverNameLen > 0 {
		if err := binary.Write(buf, binary.BigEndian, serverName); err != nil {
			return nil, err
		}
	}
	if cmdLen > 0 {
		if err := binary.Write(buf, binary.BigEndian, cmd); err != nil {
			return nil, err
		}
	}
	if bodyLen > 0 {
		if err := binary.Write(buf, binary.BigEndian, data); err != nil {
			return nil, err
		}
	}
	if err := binary.Write(buf, binary.BigEndian, PacketEnd); err != nil {
		return nil, err
	}

	return buf.Bytes(), nil
}

func newCommonHandler() *CommonHandler {
	return &CommonHandler{}
}

//启动common udp服务
func CommonListenAndServeUdp(config config.ConfigServer) {
	check := NewCommonUdpChecker()
	limit := server.NewTokenBucketLimiter(int64(config.CapacityLimit))
	handler := newCommonHandler()
	addr := fmt.Sprintf(":%d", config.Port)

	server.ListenAndServeUdp(addr, check, limit, handler,
		time.Duration(config.NetTimeOutMs)*time.Millisecond, uint64(config.CapacityPool))
}

//启动common tcp服务
func CommonListenAndServeTcp(config config.ConfigServer) {
	check := NewCommonTcpChecker()
	limit := server.NewTokenBucketLimiter(int64(config.CapacityLimit))
	handler := newCommonHandler()
	addr := fmt.Sprintf(":%d", config.Port)

	server.ListenAndServeTcp(addr, check, limit, handler,
		time.Duration(config.NetTimeOutMs)*time.Millisecond,
		time.Duration(config.IdleTimeoutMs)*time.Millisecond,
		uint64(config.CapacityPool), false)
}
