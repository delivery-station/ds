package main

import (
	"context"
	"fmt"
	"log"
	"os"

	"github.com/delivery-station/ds/pkg/client"
)

const (
	version = "1.0.0"
	name    = "my-plugin"
)

func main() {
	// Handle --version flag
	if len(os.Args) > 1 && os.Args[1] == "--version" {
		fmt.Println(version)
		os.Exit(0)
	}

	// Handle --help flag
	if len(os.Args) > 1 && (os.Args[1] == "--help" || os.Args[1] == "-h") {
		printHelp()
		os.Exit(0)
	}

	// Parse command
	if len(os.Args) < 2 {
		fmt.Fprintf(os.Stderr, "Error: No command specified\n\n")
		printHelp()
		os.Exit(1)
	}

	command := os.Args[1]
	args := os.Args[2:]

	// Create DS client
	dsClient, err := client.NewClient()
	if err != nil {
		log.Fatalf("Failed to create DS client: %v", err)
	}
	defer dsClient.Close()

	ctx := context.Background()

	// Route to command handler
	switch command {
	case "hello":
		handleHello(ctx, dsClient, args)
	case "process":
		handleProcess(ctx, dsClient, args)
	default:
		fmt.Fprintf(os.Stderr, "Error: Unknown command: %s\n\n", command)
		printHelp()
		os.Exit(1)
	}
}

func printHelp() {
	fmt.Printf(`%s - DS Plugin Template

Usage:
  ds %s <command> [options]

Commands:
  hello [name]          Say hello to someone
  process <artifact>    Process an artifact

Global Flags:
  --version            Show version
  --help, -h           Show this help

Examples:
  ds %s hello World
  ds %s process ghcr.io/myorg/app:v1.0.0

Environment Variables:
  DS_REGISTRY_DEFAULT   Default OCI registry
  DS_CACHE_DIR          Cache directory path
  DS_LOGGING_LEVEL      Log level (debug, info, warn, error)

For more information: https://github.com/delivery-station/ds
`, name, name, name, name)
}

func handleHello(ctx context.Context, dsClient *client.Client, args []string) {
	name := "World"
	if len(args) > 0 {
		name = args[0]
	}

	// Read DS configuration from environment
	registry := os.Getenv("DS_REGISTRY_DEFAULT")
	logLevel := os.Getenv("DS_LOGGING_LEVEL")

	fmt.Printf("Hello, %s!\n", name)

	if logLevel == "debug" {
		fmt.Printf("Debug: Using registry: %s\n", registry)
		fmt.Printf("Debug: Plugin version: %s\n", version)
	}

	// Example: Publish an event
	err := dsClient.PublishEvent(ctx, client.Event{
		Type:   "my-plugin.hello",
		Source: "my-plugin",
		Data: map[string]interface{}{
			"name": name,
		},
	})
	if err != nil {
		log.Printf("Warning: Failed to publish event: %v", err)
	}
}

func handleProcess(ctx context.Context, dsClient *client.Client, args []string) {
	if len(args) < 1 {
		fmt.Fprintf(os.Stderr, "Error: Missing artifact reference\n")
		fmt.Fprintf(os.Stderr, "Usage: ds %s process <artifact>\n", name)
		os.Exit(1)
	}

	artifactRef := args[0]

	// Log what we're doing
	logLevel := os.Getenv("DS_LOGGING_LEVEL")
	if logLevel == "debug" || logLevel == "info" {
		fmt.Printf("Processing artifact: %s\n", artifactRef)
	}

	// Example: Pull artifact using DS
	fmt.Printf("Pulling %s...\n", artifactRef)
	err := dsClient.Pull(ctx, artifactRef, os.Stdout)
	if err != nil {
		log.Fatalf("Failed to pull artifact: %v", err)
	}

	// Your processing logic here
	fmt.Println("Processing...")

	// Example: Store state
	err = dsClient.SetState(ctx, "my-plugin.last-processed", artifactRef)
	if err != nil {
		log.Printf("Warning: Failed to store state: %v", err)
	}

	// Example: Publish completion event
	err = dsClient.PublishEvent(ctx, client.Event{
		Type:   "my-plugin.processed",
		Source: "my-plugin",
		Data: map[string]interface{}{
			"artifact": artifactRef,
			"status":   "success",
		},
	})
	if err != nil {
		log.Printf("Warning: Failed to publish event: %v", err)
	}

	fmt.Println("Done!")
}
