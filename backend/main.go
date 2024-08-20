package main

import "github.com/spf13/pflag"

func main() {
	path := pflag.StringP("config", "c", "config.yaml", "config path")
	
}
