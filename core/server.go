// Package core 提供 AI Switch 的模块化服务器框架。
//
// Server 整合了 HTTP 路由、模块生命周期管理、优雅关闭和依赖注入。
// 所有可插拔模块（webui / cost / keymgr）通过 RegisterModule() 注册。
//
// 典型用法：
//
//	srv := core.NewServer(proxy.Config{...})
//	for _, m := range core.GetModules() {
//	    srv.RegisterModule(m)
//	}
//	srv.Run()
package core

import (
	"context"
	"fmt"
	"net/http"
	"os"
	"os/signal"
	"sync"

	"github.com/yourusername/ais/internal/config"
	"github.com/yourusername/ais/internal/keymanager"
	"github.com/yourusername/ais/internal/logger"
	"github.com/yourusername/ais/internal/proxy"
	"github.com/yourusername/ais/pkg/module"
	"github.com/yourusername/ais/pkg/price"
	"github.com/yourusername/ais/pkg/types"
)

// Server 是 AI Switch 的模块化 HTTP 服务器。
//
// 它负责：
//   - 解析和验证代理配置
//   - 构建 HTTP 路由（/v1/chat/completions、/health）
//   - 管理模块生命周期（注册 → Init → Start → Stop）
//   - 通过 KeyMgr 解析 API Key
//   - 优雅关闭（SIGINT）
//
// 依赖注入点（公开字段，测试时可替换）：
//
//	Logger   日志输出
//	KeyMgr   API Key 管理器
//	PriceTbl 价格计算器
type Server struct {
	mu      sync.Mutex
	cfg     *types.Config
	httpSrv *http.Server
	modules map[string]module.Module
	proxy   proxy.Config

	// Injected dependencies
	Logger  *logger.Logger
	KeyMgr  *keymanager.Manager
	PriceTbl *price.Table
}

// Port sets the listen port (overrides cfg.Server.Port).
func (s *Server) Port(p int) { s.cfg.Server.Port = p }

// NewServer creates a Server from a proxy config and optional typed config.
func NewServer(pcfg proxy.Config) *Server {
	s := &Server{
		cfg:     &types.Config{},
		proxy:   pcfg,
		Logger:  logger.New(),
		KeyMgr:  keymanager.New("AIS"),
		modules: make(map[string]module.Module),
	}
	s.PriceTbl = price.Global()
	return s
}

// NewServerFromConfig creates a Server from a full types.Config.
func NewServerFromConfig(cfg *types.Config) *Server {
	s := &Server{
		cfg:     cfg,
		modules: make(map[string]module.Module),
		Logger:  logger.New(),
		KeyMgr:  keymanager.New("AIS"),
	}
	s.PriceTbl = price.Global()
	return s
}

// RegisterModule initialises and registers a module.
func (s *Server) RegisterModule(m module.Module) error {
	ctx := &module.CoreContext{Config: s.cfg}
	if err := m.Init(ctx); err != nil {
		return fmt.Errorf("init module %s: %w", m.Name(), err)
	}
	s.mu.Lock()
	s.modules[m.Name()] = m
	s.mu.Unlock()
	return nil
}

// ApplyConfigProfile merges settings from a saved config.Profile.
func (s *Server) ApplyConfigProfile(cfg *config.File, name string) error {
	if cfg == nil {
		return fmt.Errorf("no config loaded")
	}
	if name == "" {
		name = cfg.DefaultProfile
	}
	if name == "" {
		return fmt.Errorf("no default profile set")
	}
	p := cfg.GetProfile(name)
	if p == nil {
		return fmt.Errorf("profile %q not found", name)
	}
	if s.proxy.Provider == "openai" && p.Provider != "" {
		s.proxy.Provider = p.Provider
	}
	if s.proxy.Key == "" && p.Key != "" {
		s.proxy.Key = p.Key
	}
	if s.proxy.Model == "" && p.Model != "" {
		s.proxy.Model = p.Model
	}
	if s.proxy.BaseURL == "" && p.BaseURL != "" {
		s.proxy.BaseURL = p.BaseURL
	}
	return nil
}

// Start begins listening and blocks until the context is cancelled.
func (s *Server) resolveKey() {
	s.proxy.KeyMgr = s.KeyMgr
	if s.proxy.Key == "" {
		if key, err := s.KeyMgr.GetKey(s.proxy.Provider); err == nil {
			s.proxy.Key = key
		}
	}
}

func (s *Server) validate() error {
	if s.proxy.Key == "" {
		return fmt.Errorf("no API key provided (use --key, AIS_%s_KEY, or `ais config`)", s.proxy.Provider)
	}
	if s.proxy.Model == "" {
		return fmt.Errorf("model is required")
	}
	return nil
}

func (s *Server) buildRoutes() *http.ServeMux {
	mux := http.NewServeMux()
	mux.HandleFunc("/v1/chat/completions", proxy.Handler(s.proxy))
	mux.HandleFunc("/health", s.handleHealth)
	mux.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		http.NotFound(w, r)
	})
	return mux
}

func (s *Server) startModules(ctx context.Context) error {
	for name, m := range s.modules {
		if !m.Enabled() {
			continue
		}
		if err := m.Start(ctx); err != nil {
			return fmt.Errorf("starting module %s: %w", name, err)
		}
	}
	return nil
}

func (s *Server) listenAddr() string {
	port := s.cfg.Server.Port
	if port == 0 {
		port = 8080
	}
	host := s.cfg.Server.Host
	if host == "" {
		host = "localhost"
	}
	return fmt.Sprintf("%s:%d", host, port)
}

// Start begins listening and blocks until the context is cancelled.
func (s *Server) Start(ctx context.Context) error {
	s.resolveKey()
	if err := s.validate(); err != nil {
		return err
	}

	mux := s.buildRoutes()
	if err := s.startModules(ctx); err != nil {
		return err
	}

	s.httpSrv = &http.Server{Addr: s.listenAddr(), Handler: mux}
	fmt.Fprintf(os.Stderr, "AI Switch started on %s\n", s.listenAddr())
	if s.proxy.Verbose {
		fmt.Fprintln(os.Stderr, "[VERBOSE] Debug mode enabled")
	}

	go func() {
		<-ctx.Done()
		s.httpSrv.Shutdown(context.Background())
	}()

	return s.httpSrv.ListenAndServe()
}

// Stop shuts down the server and all modules.
func (s *Server) Stop() error {
	s.mu.Lock()
	defer s.mu.Unlock()
	for name, m := range s.modules {
		if err := m.Stop(); err != nil {
			fmt.Fprintf(os.Stderr, "warning: stopping module %s: %v\n", name, err)
		}
	}
	if s.httpSrv != nil {
		return s.httpSrv.Close()
	}
	return nil
}

// Run is a convenience method that starts the server and waits for SIGINT.
func (s *Server) Run() error {
	ctx, cancel := signal.NotifyContext(context.Background(), os.Interrupt)
	defer cancel()
	return s.Start(ctx)
}

func (s *Server) handleHealth(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	fmt.Fprintf(w, `{"status":"ok","provider":"%s","model":"%s"}`,
		s.proxy.Provider, s.proxy.Model)
}
