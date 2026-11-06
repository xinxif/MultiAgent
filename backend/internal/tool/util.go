// 辅助小函数（避免包内循环引用散落各处）。
package tool

import "time"

// timeNowUnixMilli 当前毫秒时间戳（测试可替换）。
var timeNowUnixMilli = func() int64 {
	return time.Now().UnixMilli()
}
