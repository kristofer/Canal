//go:build tinygo

// Domain picoceci runs the picoceci language interpreter as a Canal domain.
//
// This domain provides:
//   - Interactive REPL via UART console
//   - File system access via Canal capabilities
//   - Full access to 8MB PSRAM for heap allocations
//
// Build as part of Canal:
//   make flash
package main

import (
	"time"

	"github.com/kristofer/picoceci/pkg/bytecode"
	"github.com/kristofer/picoceci/pkg/lexer"
	"github.com/kristofer/picoceci/pkg/module"
	"github.com/kristofer/picoceci/pkg/parser"
)

const version = "0.1.0-dev"

func main() {
	// Wait for USB CDC / UART to stabilize
	time.Sleep(2 * time.Second)

	println("[picoceci] Starting v" + version + " (Canal domain)")

	// Set up module resolver
	// TODO: Use Canal's stdlib/fs for file reading once capabilities are wired
	resolver := module.NewResolver(func(path string) ([]byte, error) {
		// For now, return error - will wire to Canal FS capability
		return nil, errNoFS
	})
	module.RegisterBuiltins(resolver)
	loader := module.NewLoader(resolver)

	println("[picoceci] Ready.")
	println("")

	// Start REPL
	runREPL(loader)
}

// runREPL runs the interactive REPL using Canal's console I/O.
// Uses println for output and a simple line reader for input.
func runREPL(loader *module.Loader) {
	console := newConsole()

	for {
		print("> ")

		line, err := console.ReadLine()
		if err != nil {
			println("\n[picoceci] Goodbye!")
			break
		}
		if line == "" {
			continue
		}

		// Parse
		l := lexer.NewString(line)
		p := parser.New(l)
		prog, err := p.ParseProgram()
		if err != nil {
			println("parse: " + err.Error())
			continue
		}

		// Compile
		c := bytecode.NewCompilerWithLoader(loader)
		chunk, err := c.Compile(prog.Statements)
		if err != nil {
			println("compile: " + err.Error())
			continue
		}

		// Run with fresh VM each time (memory optimization)
		vm := bytecode.NewVM()
		vm.SetBlocks(c.GetBlocks())
		vm.AddGlobals(c.GetGlobals())
		result, err := vm.Run(chunk)
		if err != nil {
			println("error: " + err.Error())
			continue
		}

		// Print result
		if result != nil {
			println("=> " + result.PrintString())
		}
	}
}

// Error types
type fsError struct{}

func (fsError) Error() string { return "filesystem not available" }

var errNoFS = fsError{}
