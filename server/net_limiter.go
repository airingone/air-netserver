package server

import (
	"sync"
	"sync/atomic"
	"time"
)
//限频handle

//默认限频 不做限频
type DefaultLimiter struct {
}

//默认不限
func (l *DefaultLimiter) Acquire() bool {
	return true
}

//创建默认limit
func NewDefaultLimiter() NetLimiter {
	return &DefaultLimiter{}
}

//TokenBucketLimiter 令牌桶算法限频器 秒级控制速率
type TokenBucketLimiter struct {
	capacity int64 //限频值
	avail    int64 //当前时间值
}

//限频判断
func (l *TokenBucketLimiter) Acquire() bool {
	if l == nil {
		return true
	}
	if atomic.AddInt64(&l.avail, 1) > l.capacity {
		return false
	}

	return true
}

//NewTokenBucketLimiter 新建一个容量为capacity的令牌桶算法限频器
//capacity: 容量
func NewTokenBucketLimiter(capacity int64) NetLimiter {
	if capacity <= 0 {
		return nil
	}
	l := &TokenBucketLimiter{capacity: capacity, avail: 0}
	go func() {
		for { //每秒重新设置值
			time.Sleep(time.Second)
			atomic.StoreInt64(&l.avail, 0)
		}
	}()

	return l
}

//LeakyBucketLimiter 漏桶算法限频器 秒级控制速率
type LeakyBucketLimiter struct {
	lastAccessTime time.Time   //上一时间点
	capacity       int64       //容量
	avail          int64       //当前量
	mu             sync.Mutex
}

//Acquire 漏桶算法限频器实现。
func (l *LeakyBucketLimiter) Acquire() bool {
	if l == nil {
		return true
	}
	l.mu.Lock()
	if l.avail > 0 {
		l.avail-- //水桶2取出水滴
		l.mu.Unlock()
		return true
	}
	//水桶2空了
	now := time.Now()
	add := l.capacity * now.Sub(l.lastAccessTime).Nanoseconds() / 1e9 //即每秒获取的水量，如果1s内以消耗完就到了这里则会add<=0
	if add > 0 {
		l.lastAccessTime = now
		l.avail += add //水桶1全部倒入水桶2
		if l.avail > l.capacity {
			l.avail = l.capacity //溢出抛弃
		}
		l.avail--
		l.mu.Unlock()
		return true
	}
	l.mu.Unlock()
	return false
}

//NewLeakyBucketLimiter 新建一个容量为capacity的漏桶算法限频器
func NewLeakyBucketLimiter(capacity int64) NetLimiter {
	if capacity <= 0 {
		return nil
	}
	return &LeakyBucketLimiter{capacity: capacity, lastAccessTime: time.Now(), avail: capacity}
}
