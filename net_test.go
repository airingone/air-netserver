package air_netserver

import (
	"context"
	"encoding/json"
	airetcd "github.com/airingone/air-etcd"
	"github.com/airingone/air-netserver/server"
	"github.com/airingone/config"
	"github.com/airingone/log"
	"testing"
	"time"
)

///////////pool测试//////////////
type TestHandler struct {
	num int32
}

func (h *TestHandler) Handle() error {
	log.Info("handle test: %d", h.num)
	time.Sleep(5 * time.Second)
	return nil
}

func TestWorkerPool(t *testing.T) {
	config.InitConfig()                     //配置文件初始化
	log.InitLog(config.GetLogConfig("log")) //日志初始化

	pool := server.NewWorkPool(20)
	for i := 0; i < 20; i++ {
		handle := &TestHandler{
			num: int32(i),
		}
		err := pool.Put(handle)
		if err != nil { //这里调整capacity可以观察失败请求，基本capacity为执行任务的一半就不会有put错误，因为chan有capacity大小队列
			log.Info("put err: %+v", i)
		}
	}
	time.Sleep(10 * time.Second)
	log.Info("working num: %d", pool.GetWorkerNumber())
	select {}
	pool.Close()
}

///////////common udp server测试//////////////
type GetUserInfoReq struct {
	RequestId string `json:"request_id"`
	UserId    string `json:"user_id"`
}

type GetUserInfoRsp struct {
	RequestId string `json:"request_id"`
	ErrCode   int32  `json:"err_code"`
	ErrMsg    string `json:"err_msg"`
	UserId    string `json:"user_id"`
	UserName  string `json:"user_name"`
}

func ResponseErr(requestId string, errCode int32, errMsg string) ([]byte, error) {
	var rsp GetUserInfoRsp
	rsp.RequestId = requestId
	rsp.ErrCode = errCode
	rsp.ErrMsg = errMsg

	rspData, _ := json.Marshal(rsp)
	return rspData, nil
}

func ResponseSucc(rsp interface{}) ([]byte, error) {
	rspData, _ := json.Marshal(rsp)
	return rspData, nil
}

//业务处理函数，每个网络请求命令分别实现一个handle即可
func HandleGetUserInfo(ctx context.Context, reqData []byte) ([]byte, error) {
	//解析请求包数据，可以是json，pb，string等等,其中具体请求协议由业务自行制定(可以固定req与rsp head)，这里已json为例
	var req GetUserInfoReq
	err := json.Unmarshal(reqData, &req)
	if err != nil {
		return ResponseErr(req.RequestId, 10001, "para err")
	}

	var rsp GetUserInfoRsp
	rsp.RequestId = req.RequestId
	rsp.ErrCode = 0
	rsp.ErrMsg = "succ"
	rsp.UserId = req.UserId
	rsp.UserName = "testuser01"
	return ResponseSucc(rsp)
}

//udp server测试
func TestCommonUdpServer(t *testing.T) {
	config.InitConfig()                     //配置文件初始化
	log.InitLog(config.GetLogConfig("log")) //日志初始化
	airetcd.RegisterLocalServerToEtcd(config.GetString("server.name"),
		config.GetUInt32("server.port"), config.GetStringSlice("etcd.addrs")) //将服务注册到etcd集群

	//注册服务handler
	config := config.GetServerConfig("server")
	RegisterCommonFuncHandler(config.Name, "getuserinfo", HandleGetUserInfo)
	CommonListenAndServeUdp(config) //启动服务
}
