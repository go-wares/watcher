// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.
//
// author: wsfuyibing <websearch@163.com>
// date: 2023-06-21

package watcher

import (
	"context"
	"fmt"
	"os"
	"sync"
	"time"
)

const (
	// 检测频率.
	// 默认每隔3秒检查一次文件是否发生变更.
	defaultFileDuration = time.Second * 3
)

var (
	errFileIsDirectory = fmt.Errorf("specifed path is directory")
)

type (
	// File
	// 文件观察.
	File interface {
		// Add
		// 添加观察对象.
		Add(path string, notifies ...Notifier) File

		// Del
		// 删除被观察对象.
		Del(path string) File

		// SetDuration
		// 设置观察频率.
		SetDuration(duration time.Duration) File

		// Start
		// 启动观察.
		Start(ctx context.Context)

		// Stop
		// 退出观察
		Stop()
	}

	// 文件结构体.
	file struct {
		cancel            context.CancelFunc
		ctx               context.Context
		duration          time.Duration
		modifies          map[string]time.Time
		mu                sync.RWMutex
		registries        map[string][]Notifier
		started, scanning bool
	}
)

// NewFile
// 创建文件观察器.
func NewFile() File {
	return (&file{
		duration: defaultFileDuration,
	}).init()
}

// Add
// 添加观察对象.
func (o *file) Add(path string, notifies ...Notifier) File {
	o.mu.Lock()
	defer o.mu.Unlock()

	if _, ok := o.registries[path]; !ok {
		o.registries[path] = make([]Notifier, 0)
	}

	o.registries[path] = append(o.registries[path], notifies...)
	return o
}

// Del
// 删除被观察对象.
func (o *file) Del(path string) File {
	o.mu.Lock()
	defer o.mu.Unlock()

	if _, ok := o.registries[path]; ok {
		delete(o.modifies, path)
		delete(o.registries, path)
	}

	return o
}

// SetDuration
// 设置观察频率.
func (o *file) SetDuration(duration time.Duration) File {
	o.mu.Lock()
	defer o.mu.Unlock()
	o.duration = duration
	return o
}

// Start
// 启动观察.
func (o *file) Start(ctx context.Context) {
	o.mu.Lock()

	// 1. 重复启动.
	if o.started {
		o.mu.Unlock()
		return
	}

	// 2. 启动过程.
	if ctx == nil {
		ctx = context.Background()
	}

	o.ctx, o.cancel = context.WithCancel(ctx)
	o.started = true
	o.mu.Unlock()

	// 3. 监听退出.
	//    当服务退出时, 恢复初始状态.
	defer func() {
		o.mu.Lock()
		defer o.mu.Unlock()
		o.ctx = nil
		o.cancel = nil
		o.started = false
		o.scanning = false
	}()

	// 4. 立即检测.
	o.scan()

	// 5. 定时检测.
	ti := time.NewTicker(o.duration)
	for {
		select {
		case <-ti.C:
			go o.scan()
		case <-o.ctx.Done():
			ti.Stop()
			return
		}
	}
}

// Stop
// 退出观察
func (o *file) Stop() {
	o.mu.RLock()
	defer o.mu.RUnlock()

	if o.ctx != nil && o.ctx.Err() == nil {
		o.cancel()
	}
}

// /////////////////////////////////////////////////////////////////////////////
// Access methods
// /////////////////////////////////////////////////////////////////////////////

// 构造实例.
func (o *file) init() *file {
	o.modifies = make(map[string]time.Time)
	o.registries = make(map[string][]Notifier)
	return o
}

// 读取文件.
func (o *file) read(path string, notifies []Notifier) {
	var (
		body    []byte
		changed = false
		err     error
		info    os.FileInfo
	)

	// 1. 结束读取.
	defer func() {
		// 捕获异常.
		if v := recover(); v != nil {
			err = fmt.Errorf("%v", v)
		}

		// 读取出错.
		if err != nil || changed {
			for _, notifier := range notifies {
				go notifier(path, body, err)
			}
		}
	}()

	// 2. 文件检测.
	if info, err = os.Stat(path); err != nil {
		return
	}

	// 2.1 禁止目录.
	if info.IsDir() {
		err = errFileIsDirectory
		return
	}

	// 2.2 变更检测.
	if func() bool {
		if t, ok := o.modifies[path]; ok && t.UnixMilli() == info.ModTime().UnixMilli() {
			return true
		}
		return false
	}() {
		return
	}

	// 3. 读取内容.
	body, err = os.ReadFile(path)
	changed = true

	// 3.1 记录时间.
	o.mu.Lock()
	o.modifies[path] = info.ModTime()
	o.mu.Unlock()
}

// 扫描文件.
func (o *file) scan() {
	o.mu.Lock()

	if o.scanning {
		o.mu.Unlock()
		return
	}

	o.scanning = true
	o.mu.Unlock()

	w := sync.WaitGroup{}
	defer func() {
		w.Wait()
		o.mu.Lock()
		o.scanning = false
		o.mu.Unlock()
	}()

	for k, vs := range func() map[string][]Notifier {
		o.mu.RLock()
		defer o.mu.RUnlock()
		return o.registries
	}() {
		w.Add(1)
		go func(path string, notifies []Notifier) {
			defer w.Done()
			o.read(path, notifies)
		}(k, vs)
	}
}
