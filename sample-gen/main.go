package main

import (
	"bufio"
	"log"
	"os"
	"strings"

	"stock-forecast-backend/pkg/samplegen"
)

func loadEnvFile(filename string) bool {
	file, err := os.Open(filename)
	if err != nil {
		return false
	}
	defer file.Close()

	scanner := bufio.NewScanner(file)
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		parts := strings.SplitN(line, "=", 2)
		if len(parts) == 2 {
			os.Setenv(strings.TrimSpace(parts[0]), strings.TrimSpace(parts[1]))
		}
	}
	return true
}

func init() {
	_ = loadEnvFile(".env")
	_ = loadEnvFile(".env.local")
}

func main() {
	if err := samplegen.Execute(os.Args[1:]); err != nil {
		log.Fatalf("sample-gen failed: %v", err)
	}
}
