package main

import (
	"flag"
	"fmt"
	"net"
	"os"
	"strconv"
	"strings"
	"time"
)

const defaultAddress = "127.0.0.1:19081"

type config struct {
	address       string
	dataDirectory string
	selfcheck     bool
	selfcheckTTL  time.Duration
	dataDirSet    bool
}

func parseConfig(arguments []string, getenv func(string) string) (config, error) {
	flags := flag.NewFlagSet("radio-coordination-server", flag.ContinueOnError)
	flags.SetOutput(os.Stderr)
	address := flags.String("addr", defaultAddress, "HTTP 监听地址")
	dataDirectory := flags.String("data-dir", "data", "本地账本与投影目录")
	selfcheck := flags.Bool("selfcheck", false, "运行端到端自检后退出")
	selfcheckTTL := flags.Duration("selfcheck-timeout", 20*time.Second, "自检总超时")
	if err := flags.Parse(arguments); err != nil {
		return config{}, err
	}
	if flags.NArg() != 0 {
		return config{}, fmt.Errorf("不支持位置参数: %s", strings.Join(flags.Args(), " "))
	}
	addressSet := false
	dataDirSet := false
	flags.Visit(func(item *flag.Flag) {
		if item.Name == "addr" {
			addressSet = true
		}
		if item.Name == "data-dir" {
			dataDirSet = true
		}
	})
	resolvedAddress := *address
	if !addressSet {
		if rawPort := strings.TrimSpace(getenv("PORT")); rawPort != "" {
			port, err := strconv.Atoi(rawPort)
			if err != nil || port < 1 || port > 65535 {
				return config{}, fmt.Errorf("PORT 必须是 1 至 65535 的端口号")
			}
			resolvedAddress = net.JoinHostPort("127.0.0.1", strconv.Itoa(port))
		}
	}
	if err := validateAddress(resolvedAddress); err != nil {
		return config{}, err
	}
	if strings.TrimSpace(*dataDirectory) == "" {
		return config{}, fmt.Errorf("data-dir 不能为空")
	}
	if *selfcheckTTL <= 0 || *selfcheckTTL > 2*time.Minute {
		return config{}, fmt.Errorf("selfcheck-timeout 必须大于 0 且不超过 2 分钟")
	}
	return config{address: resolvedAddress, dataDirectory: *dataDirectory, selfcheck: *selfcheck, selfcheckTTL: *selfcheckTTL, dataDirSet: dataDirSet}, nil
}

func validateAddress(address string) error {
	host, portText, err := net.SplitHostPort(address)
	if err != nil {
		return fmt.Errorf("addr 必须是 host:port: %w", err)
	}
	port, err := strconv.Atoi(portText)
	if err != nil || port < 1 || port > 65535 {
		return fmt.Errorf("addr 端口必须处于 1 至 65535")
	}
	ip := net.ParseIP(host)
	if ip == nil || !ip.IsLoopback() {
		return fmt.Errorf("addr 必须使用明确的回环 IP，不能绑定非回环接口")
	}
	return nil
}
