package main

import (
	"log"
	"os"
)

func main() {
	log.SetFlags(log.Ldate | log.Ltime | log.LUTC)
	configuration, err := parseConfig(os.Args[1:], os.Getenv)
	if err != nil {
		log.Printf("配置错误: %v", err)
		os.Exit(2)
	}
	if err := run(configuration); err != nil {
		log.Printf("服务退出: %v", err)
		os.Exit(1)
	}
}
