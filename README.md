# 网络srver组件
## 1.组件描述
网络srver组件实现了udp与tcp服务端组件，在写后台服务是使用方不需关注网络等处理，只需要写业务处理handler即可。网络包协议这里实现了common_server，如果有特殊要求可以自行实现check handler与数据包解析。

## 2.如何使用
### 2.1 udp server
```
import (
    "github.com/airingone/config"
    "github.com/airingone/log"
    airetcd "github.com/airingone/air-etcd"
    air_netserver "github.com/airingone/air-netserver"
)

func main() {
    config.InitConfig()                        //进程启动时调用一次初始化配置文件，配置文件名为config.yml，目录路径为../conf/或./
    log.InitLog(config.GetLogConfig("log"))    //进程启动时调用一次初始化日志
    airetcd.RegisterLocalServerToEtcd(config.GetString("server.name"),
        config.GetUInt32("server.port"), config.GetStringSlice("etcd.addrs")) //将服务注册到etcd集群

    //注册服务handler
    config := config.GetServerConfig("server")
    air_netserver.RegisterCommonFuncHandler(config.Name, "getuserinfo", HandleGetUserInfo)

    air_netserver.CommonListenAndServeUdp(config) //启动服务  
}
```
[handler实现请见net_test.go](https://github.com/airingone/air-netserver/blob/master/net_test.go)
### 2.2 tcp server
```
import (
    "github.com/airingone/config"
    "github.com/airingone/log"
    airetcd "github.com/airingone/air-etcd"
    air_netserver "github.com/airingone/air-netserver"
)

func main() {
    config.InitConfig()                        //进程启动时调用一次初始化配置文件，配置文件名为config.yml，目录路径为../conf/或./
    log.InitLog(config.GetLogConfig("log"))    //进程启动时调用一次初始化日志
    airetcd.RegisterLocalServerToEtcd(config.GetString("server.name"),
        config.GetUInt32("server.port"), config.GetStringSlice("etcd.addrs")) //将服务注册到etcd集群

    //注册服务handler
    config := config.GetServerConfig("server")
    log.Info("config: %+v", config)
    air_netserver.RegisterCommonFuncHandler(config.Name, "getuserinfo", HandleGetUserInfo)
    
    air_netserver.CommonListenAndServeTcp(config) //启动服务 
}
```
[handler实现请见net_test.go](https://github.com/airingone/air-netserver/blob/master/net_test.go)
