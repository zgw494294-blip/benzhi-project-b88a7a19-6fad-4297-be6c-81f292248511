package main

import (
	"context"
	"errors"
	"fmt"
	"log"
	"net"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"benzhi-project-b88a7a19-6fad-4297-be6c-81f292248511/internal/analysis"
	"benzhi-project-b88a7a19-6fad-4297-be6c-81f292248511/internal/coordination"
	"benzhi-project-b88a7a19-6fad-4297-be6c-81f292248511/internal/httpapi"
	"benzhi-project-b88a7a19-6fad-4297-be6c-81f292248511/internal/store"
)

func run(configuration config) error {
	dataDirectory := configuration.dataDirectory
	cleanup := func() {}
	if configuration.selfcheck && !configuration.dataDirSet {
		temporary, err := os.MkdirTemp("", "radio-coordination-selfcheck-")
		if err != nil {
			return fmt.Errorf("创建自检数据目录失败: %w", err)
		}
		dataDirectory = temporary
		cleanup = func() { _ = os.RemoveAll(temporary) }
	}
	defer cleanup()
	repository, err := store.Open(dataDirectory)
	if err != nil {
		return fmt.Errorf("初始化本地事实账本失败: %w", err)
	}
	engine := analysis.NewEngine()
	service := coordination.NewService(repository, engine)
	handler := httpapi.New(service)
	listener, err := net.Listen("tcp", configuration.address)
	if err != nil {
		return fmt.Errorf("监听 %s 失败: %w", configuration.address, err)
	}
	server := &http.Server{
		Handler: handler, ReadHeaderTimeout: 5 * time.Second, ReadTimeout: 10 * time.Second,
		WriteTimeout: 15 * time.Second, IdleTimeout: 60 * time.Second,
	}
	serveErrors := make(chan error, 1)
	go func() {
		err := server.Serve(listener)
		if err != nil && !errors.Is(err, http.ErrServerClosed) {
			serveErrors <- err
			return
		}
		serveErrors <- nil
	}()
	if configuration.selfcheck {
		return runSelfcheck(server, serveErrors, listener.Addr().String(), configuration.selfcheckTTL)
	}
	log.Printf("无线电协调服务已监听 http://%s", listener.Addr().String())
	return waitForShutdown(server, serveErrors)
}

func runSelfcheck(server *http.Server, serveErrors <-chan error, address string, timeout time.Duration) error {
	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()
	checkResult := make(chan error, 1)
	go func() { checkResult <- httpapi.RunSelfcheck(ctx, "http://"+address) }()
	var checkErr error
	select {
	case checkErr = <-checkResult:
	case serveErr := <-serveErrors:
		if serveErr != nil {
			checkErr = fmt.Errorf("自检期间 HTTP 服务异常退出: %w", serveErr)
		} else {
			checkErr = fmt.Errorf("自检完成前 HTTP 服务已经关闭")
		}
	case <-ctx.Done():
		checkErr = fmt.Errorf("自检超时: %w", ctx.Err())
	}
	shutdownCtx, shutdownCancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer shutdownCancel()
	shutdownErr := server.Shutdown(shutdownCtx)
	if checkErr != nil {
		return checkErr
	}
	if shutdownErr != nil {
		return fmt.Errorf("自检后关闭 HTTP 服务失败: %w", shutdownErr)
	}
	select {
	case serveErr := <-serveErrors:
		if serveErr != nil {
			return fmt.Errorf("HTTP 服务退出失败: %w", serveErr)
		}
	case <-time.After(5 * time.Second):
		return fmt.Errorf("HTTP 服务未在自检后及时退出")
	}
	log.Printf("自检通过：已完成退回、修订、重审、冻结、授权和验真")
	return nil
}

func waitForShutdown(server *http.Server, serveErrors <-chan error) error {
	signalContext, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()
	select {
	case err := <-serveErrors:
		if err != nil {
			return fmt.Errorf("HTTP 服务异常退出: %w", err)
		}
		return nil
	case <-signalContext.Done():
	}
	shutdownContext, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	if err := server.Shutdown(shutdownContext); err != nil {
		return fmt.Errorf("优雅关闭失败: %w", err)
	}
	if err := <-serveErrors; err != nil {
		return fmt.Errorf("HTTP 服务退出失败: %w", err)
	}
	return nil
}
