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
	"regexp"
	"sync"
	"time"
)

var (
	emptyTime = time.Unix(0, 0)
)

const (
	defaultFileTicker = time.Second * 3
)

type (
	// FileWatcher
	// 文件观察.
	FileWatcher interface {
		// Add
		// 添加文件.
		Add(paths ...string) (err error)

		// AddDir
		// 添加目录.
		AddDir(paths ...string) (err error)

		Start(ctx context.Context) (err error)
		Stop()

		WithFilter(filter *regexp.Regexp) FileWatcher
		WithNotifier(notifier Notifier) FileWatcher
	}

	fw struct {
		cancel   context.CancelFunc
		ctx      context.Context
		dirs     []string
		files    map[string]int64
		filter   *regexp.Regexp
		mu       *sync.RWMutex
		notifier Notifier
	}
)

func NewFile() FileWatcher {
	return &fw{
		dirs:  make([]string, 0),
		files: make(map[string]int64),
		mu:    &sync.RWMutex{},
	}
}

func (o *fw) Add(paths ...string) (err error) {
	for _, path := range paths {
		if err = o.addFile(path); err != nil {
			return err
		}
	}
	return
}

func (o *fw) AddDir(paths ...string) (err error) {
	for _, path := range paths {
		if err = o.scan(path); err != nil {
			return
		}
	}
	return
}

func (o *fw) Start(ctx context.Context) (err error) {
	o.mu.Lock()

	// 1. 重复启动.
	if o.ctx != nil && o.ctx.Err() == nil {
		o.mu.Unlock()
		return
	}

	// 2. 锁定状态.
	o.ctx, o.cancel = context.WithCancel(ctx)
	o.mu.Unlock()

	// 3. 恢复状态.
	defer func() {
		o.mu.Lock()
		defer o.mu.Unlock()
		o.ctx = nil
		o.cancel = nil
	}()

	// 4. 预先加载.
	if err = o.reader(); err != nil {
		return
	}

	// 5. 监听信号.
	var ticker = time.NewTicker(defaultFileTicker)

	for {
		select {
		case <-ticker.C:
			go func() {
				if o.scanner() == nil {
					_ = o.reader()
				}
			}()
		case <-ctx.Done():
			ticker.Stop()
			return
		}
	}
}

func (o *fw) Stop() {
	o.mu.RLock()
	defer o.mu.RUnlock()
	if o.ctx != nil && o.ctx.Err() == nil {
		o.cancel()
	}
}

func (o *fw) WithFilter(filter *regexp.Regexp) FileWatcher {
	o.mu.Lock()
	defer o.mu.Unlock()
	o.filter = filter
	return o
}

func (o *fw) WithNotifier(notifier Notifier) FileWatcher {
	o.mu.Lock()
	defer o.mu.Unlock()
	o.notifier = notifier
	return o
}

// /////////////////////////////////////////////////////////////////////////////
// Access methods
// /////////////////////////////////////////////////////////////////////////////

func (o *fw) addFile(path string) (err error) {
	o.mu.Lock()
	defer o.mu.Unlock()

	// 1. 重复添加.
	if _, ok := o.files[path]; ok {
		return
	}

	// 2. 加入观察.
	o.files[path] = 0
	return
}

func (o *fw) read(path string, ms int64) (err error) {
	var (
		info os.FileInfo
		n    = Notification{
			Path: path,
		}
	)

	defer func() {
		if o.notifier != nil && n.Type != NotChanged {
			go o.notifier(n)
		}
	}()

	// 1. 状态检查.
	if info, n.Err = os.Stat(path); n.Err != nil {
		// 1.1 文件丢失.
		if os.IsNotExist(n.Err) {
			n.Err = nil
			n.Type = NotFound
			return
		}

		// 1.2 检查出错.
		n.Type = StatFailed
		return
	}

	// 2. 从未变更.
	if info.ModTime().UnixMilli() == ms {
		n.Type = NotChanged
		return
	}

	// 3. 读取文件.
	if n.Body, n.Err = os.ReadFile(path); n.Err != nil {
		n.Type = ReadFailed
	} else {
		n.Type = ReadSuccess

		o.mu.Lock()
		defer o.mu.Unlock()
		o.files[path] = info.ModTime().UnixMilli()
	}
	return
}

func (o *fw) reader() (err error) {
	for path, ms := range func() map[string]int64 {
		o.mu.RLock()
		defer o.mu.RUnlock()
		return o.files
	}() {
		if err = o.read(path, ms); err != nil {
			return
		}
	}
	return
}

func (o *fw) scan(path string) (err error) {
	var ds []os.DirEntry

	// 1. 读取目录.
	if ds, err = os.ReadDir(path); err != nil {
		return
	}

	// 2. 遍历文件.
	for _, d := range ds {
		if d.Name() == "." || d.Name() == ".." {
			continue
		}

		k := fmt.Sprintf("%s/%s", path, d.Name())

		// 2.1 目录递归.
		if d.IsDir() {
			if err = o.scan(k); err != nil {
				return err
			}
			continue
		}

		// 2.3 发现文件.
		if o.filter != nil && !o.filter.MatchString(d.Name()) {
			continue
		}
		if err = o.addFile(k); err != nil {
			return
		}
	}

	return
}

func (o *fw) scanner() (err error) {
	for _, path := range func() []string {
		o.mu.RLock()
		defer o.mu.RUnlock()
		return o.dirs
	}() {
		if err = o.scan(path); err != nil {
			return
		}
	}
	return
}
