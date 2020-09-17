package air_netserver

import (
	"github.com/airingone/air-netserver/server"
	"github.com/airingone/config"
	"github.com/airingone/log"
	"testing"
	"time"
)

type TestHandler struct {
	num int32
}

func (h *TestHandler) Handle() error {
	log.Info("handle test: %d", h.num)
	time.Sleep(5 * time.Second)
	return nil
}

//pool测试
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
