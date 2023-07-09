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

type (
	// Notification
	// 通知结构.
	Notification struct {
		// 文件内容.
		Body []byte

		// 错误原因.
		Err error

		// 文件路径.
		Path string

		// 通知类型.
		Type NotifierType
	}

	// Notifier
	// 通知回调.
	//
	// 当目标(被观察)对象发生变更后, 触发此回调.
	Notifier func(notification Notification)

	// NotifierType
	// 通知类型.
	NotifierType int
)

const (
	_ NotifierType = iota

	NotChanged
	NotFound
	StatFailed
	ReadFailed
	ReadSuccess
)
