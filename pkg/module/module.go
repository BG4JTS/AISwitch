// Package module 定义了 AI Switch 可插拔模块的标准接口。
//
// 每个模块通过 init() 自动注册到 core 包的全局注册表中。
// 模块是否编译由 build tag 控制（webui / cost / keymgr）。
//
// 生命周期：
//   Init   → 读取配置、初始化内部状态
//   Start  → 启动后台任务、注册 HTTP 路由
//   Stop   → 优雅关闭、释放资源
package module

import (
	"context"

	"github.com/BG4JTS/AISwitch/pkg/types"
)

// CoreContext 是模块初始化时注入的共享依赖容器。
//
// 目前仅包含应用配置的引用。未来可以扩展为
// 注入 Logger、KeyManager、PriceTable 等全局单例。
type CoreContext struct {
	Config *types.Config
}

// Module 是所有可插拔模块必须实现的接口。
//
// 实现者通常在自己的 package init() 中调用
// core.RegisterModule(self) 完成注册。
type Module interface {
	// Name 返回模块的唯一标识符（如 "webui"、"cost"）。
	Name() string

	// Init 在模块注册后被调用，用于读取配置和初始化内部状态。
	// 如果返回错误，模块将被跳过。
	Init(ctx *CoreContext) error

	// Start 在服务器启动时被调用，可以启动后台 goroutine 或注册路由。
	// ctx 会在服务器关闭时被取消。
	Start(ctx context.Context) error

	// Stop 在服务器关闭时被调用，用于释放资源和等待后台任务结束。
	Stop() error

	// Requires 返回该模块依赖的模块名列表（当前未强制校验）。
	Requires() []string

	// Enabled 返回模块是否处于活跃状态。
	// 如果返回 false，Start 和 Stop 不会被调用。
	Enabled() bool
}
