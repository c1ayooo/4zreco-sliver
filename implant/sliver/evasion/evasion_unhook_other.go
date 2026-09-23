//go:build !windows

package evasion

// 非 windows 平台：无 ntdll，R-2 v1 不实现——与 sleepcrypt R-1a 的「非 windows no-op」
// 同一取舍（G-3 动态决策下 linux 免杀压力低；云工作负载 EDR 由决策链降档）。
// 调用方（runner 启动早期）自带 recover 静默且忽略返回值，no-op 即安全回落；
// 唯一引用点不区分平台（runner.go 无 build 约束），缺本 stub 会使 linux/darwin/freebsd
// 植入体整体编译失败（2026-09-24 实测：undefined: evasion.UnhookNtdll）。

// UnhookNtdll 非 windows 平台 no-op：无 ntdll 即无需用户态自恢复，无事可做也无错可报。
func UnhookNtdll() error { return nil }
