package main

import (
	"log"
	"os"

	"github.com/brngle/files"
)

func main() {
	configPath := os.Getenv("CONFIG")
	if configPath == "" && len(os.Args) > 1 {
		configPath = os.Args[1]
	}

	if configPath == "" {
		configPath = "config.hcl"
	}

	config, err := files.LoadConfig(configPath)
	if err != nil {
		log.Panicf("Failed to load configuration from path '%s': %v", configPath, err)
	}

	server := files.NewServer(config)
	panic(server.Run())
}
