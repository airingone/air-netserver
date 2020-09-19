module github.com/airingone/air-netserver

go 1.13

require (
	github.com/airingone/air-etcd v1.0.4
	github.com/airingone/air-netclient v0.0.0-20200915151055-b56937ce7000
	github.com/airingone/config v1.0.8
	github.com/airingone/log v1.0.1
	github.com/gogo/protobuf v1.3.1
)

replace github.com/airingone/config v1.0.8 => /Users/air/go/src/airingone/config

replace github.com/airingone/air-etcd v1.0.4 => /Users/air/go/src/airingone/air-etcd

replace github.com/airingone/air-netclient v0.0.0-20200915151055-b56937ce7000 => /Users/air/go/src/airingone/air-netclient
