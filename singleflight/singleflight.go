package singleflight

import "sync"

// call 代表正在进行或者已结束的请求
type call struct {
	wg  sync.WaitGroup
	val interface{}
	err error
}

// Group is the main data structure of singleFlight.
type Group struct {
	m sync.Map
}

// Do 执行函数
func (g *Group) Do(key string, fn func() (interface{}, error)) (interface{}, error) {
	// 检查是否已经存在ongoing的key
	if existing, ok := g.m.Load(key); ok {
		c := existing.(*call)
		c.wg.Wait() // 等待现在的请求完成
		return c.val, c.err
	}

	// 如果不存在
	c := &call{}
	c.wg.Add(1)
	g.m.Store(key, c)

	// execute
	c.val, c.err = fn()
	c.wg.Done()

	g.m.Delete(key)

	return c.val, c.err
}
