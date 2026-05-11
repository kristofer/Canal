//go:build tinygo

// Domain picoceci runs the picoceci language interpreter as a Canal domain.
//
// This domain provides:
//   - Interactive REPL via UART console
//   - File system access via Canal capabilities
//   - Full access to 8MB PSRAM for heap allocations
//
// Build as part of Canal:
//
//	make flash
package main

import (
	"strings"
	"unsafe"

	"github.com/kristofer/picoceci/pkg/bytecode"
	"github.com/kristofer/picoceci/pkg/lexer"
	"github.com/kristofer/picoceci/pkg/module"
	"github.com/kristofer/picoceci/pkg/parser"
)

const version = "0.1.0-dev"

// domain_entry is called by the kernel's ELF loader via xTaskCreate.
//
//export domain_entry
func domain_entry(param unsafe.Pointer) {
	// Initialize TinyGo leaking GC heap before any allocation.
	// The normal TinyGo startup path is bypassed when running as a FreeRTOS task.
	initDomainHeap()
	initDomainConsole()

	var domainID uint16
	if param != nil {
		domainID = *(*uint16)(param)
	}
	println("[picoceci] Domain", domainID, "starting from flash")
	runPicoceci()
}

func main() {
	runPicoceci()
}

func runPicoceci() {
	println("[picoceci] Starting v" + version + " (Canal domain)")

	// Set up module resolver
	resolver := module.NewResolver(func(path string) ([]byte, error) {
		return ReadFile(path)
	})
	module.RegisterBuiltins(resolver)
	loader := module.NewLoader(resolver)

	println("[picoceci] Ready.")
	println("  tip: type '---' to enter/exit paste mode for multi-line programs")
	println("")

	// Start REPL
	runREPL(loader)

	// Domain tasks must never return to FreeRTOS.
	for {
		vTaskDelay(portMAX_DELAY)
	}
}

// runREPL runs the interactive REPL using Canal's console I/O.
// Supports paste mode using '---' delimiters for multi-line programs.
func runREPL(loader *module.Loader) {
	console := newConsole()
	var buf strings.Builder
	inPaste := false

	for {
		if inPaste {
			console.Print("... ")
		} else {
			console.Print("> ")
		}

		line, err := console.ReadLine()
		if err != nil {
			if _, ok := err.(*eofError); ok {
				console.Println("")
				return
			}
			vTaskDelay(10)
			continue
		}

		if line == "---" {
			if !inPaste {
				inPaste = true
				buf.Reset()
				console.Println("(paste mode on: type '---' to run)")
			} else {
				inPaste = false
				src := buf.String()
				buf.Reset()
				if src != "" {
					evalREPLSource(loader, src)
				}
			}
			continue
		}

		if inPaste {
			buf.WriteString(line)
			buf.WriteByte('\n')
			continue
		}

		if line == "" {
			continue
		}

		evalREPLSource(loader, line)
	}
}

func evalREPLSource(loader *module.Loader, src string) {
	// Parse
	l := lexer.NewString(src)
	p := parser.New(l)
	prog, err := p.ParseProgram()
	if err != nil {
		println("parse: " + err.Error())
		return
	}

	// Compile
	c := bytecode.NewCompilerWithLoader(loader)
	chunk, err := c.Compile(prog.Statements)
	if err != nil {
		println("compile: " + err.Error())
		return
	}

	// Run with fresh VM each time (memory optimization)
	vm := bytecode.NewVM()
	vm.SetBlocks(c.GetBlocks())
	vm.AddGlobals(c.GetGlobals())
	result, err := vm.Run(chunk)
	if err != nil {
		println("error: " + err.Error())
		return
	}

	// Print result
	if result != nil {
		println("=> " + result.PrintString())
	}
}

// Error types
type fsError struct{}

func (fsError) Error() string { return "filesystem not available" }

var errNoFS = fsError{}
