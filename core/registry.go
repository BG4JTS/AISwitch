// Package core 提供 AI Switch 的模块化服务器框架。
//
// 本包包含两个核心组件：
//   - Server   — 模块化 HTTP 服务器（在 server.go 中定义）
//   - Registry — 全局模块注册表（在 registry.go 中定义）
//
// Server 整合了 HTTP 路由、模块生命周期管理、优雅关闭和依赖注入。
// 所有可插拔模块通过 RegisterModule() 注册到全局注册表中，
// Server 在启动时自动激活 Enabled() 返回 true 的模块。
//
// 模块通过 init() 函数自动注册，无需手动导入：
//
//	import _ "github.com/BG4JTS/AISwitch/modules/webui" // 自动注册
//
// 典型的服务启动流程：
//
//	srv := core.NewServer(proxy.Config{...})
//	srv.Port(8080)
//	for _, m := range core.GetModules() {
//	    srv.RegisterModule(m)
//	}
//	srv.Run()
package core

import "github.com/BG4JTS/AISwitch/pkg/module"

// registeredModules 存储所有通过 RegisterModule() 注册的模块。
// 键为模块名称（module.Name()），值为模块实例。
// 该映射仅在 init() 阶段写入，运行时只读，无需加锁。
var registeredModules = map[string]module.Module{}

// RegisterModule 向全局注册表中注册一个模块。
//
// 模块通常应在其 package init() 函数中调用此方法：
//
//	func init() {
//	    core.RegisterModule(&WebUI{})
//	}
//
// 如果多个模块注册了相同的 Name()，后者会覆盖前者（通常不应发生）。
func RegisterModule(m module.Module) {
	registeredModules[m.Name()] = m
}

// GetModules 返回注册表的完整副本。
//
// 调用方可遍历所有已注册模块，根据 Enabled() 决定是否激活。
// 返回的 map 是独立副本，对它的修改不会影响全局注册表。
func GetModules() map[string]module.Module {
	copy := make(map[string]module.Module, len(registeredModules))
	for k, v := range registeredModules {
		copy[k] = v
	}
	return copy
}

// GetModule 按名称查找已注册的模块。
//
// 如果找到则返回模块和 true，否则返回 nil 和 false。
// 典型用法：
//
//	if m, ok := core.GetModule("webui"); ok && m.Enabled() { ... }
func GetModule(name string) (module.Module, bool) {
	m, ok := registeredModules[name]
	return m, ok
}
