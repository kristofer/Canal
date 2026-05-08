//go:build tinygo && esp32s3

package main

import (
	"strings"

	"github.com/kristofer/picoceci/pkg/bytecode"
	"github.com/kristofer/picoceci/pkg/lexer"
	"github.com/kristofer/picoceci/pkg/module"
	"github.com/kristofer/picoceci/pkg/parser"
)

const picoceciVersion = "0.1.0-dev"

// runREPL runs the interactive REPL using the provided console I/O.
// Supports paste mode using '---' delimiters for multi-line programs.
func runREPL(console consoleIO) {
	// Set up module resolver
	resolver := module.NewResolver(func(path string) ([]byte, error) {
		// For now, return error - will wire to Canal FS capability
		return nil, errNoFS
	})
	module.RegisterBuiltins(resolver)
	loader := module.NewLoader(resolver)

	console.Println("[picoceci] Ready v" + picoceciVersion)
	console.Println("  tip: type '---' to enter/exit paste mode for multi-line programs")
	console.Println("")

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
					evalREPLSource(console, loader, src)
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

		evalREPLSource(console, loader, line)
	}
}

func evalREPLSource(console consoleIO, loader *module.Loader, src string) {
	// Parse
	l := lexer.NewString(src)
	p := parser.New(l)
	prog, err := p.ParseProgram()
	if err != nil {
		console.Println("parse: " + err.Error())
		return
	}

	// Compile
	c := bytecode.NewCompilerWithLoader(loader)
	chunk, err := c.Compile(prog.Statements)
	if err != nil {
		console.Println("compile: " + err.Error())
		return
	}

	// Run with fresh VM each time (memory optimization)
	vm := bytecode.NewVMWithTranscript(console)
	vm.SetBlocks(c.GetBlocks())
	vm.AddGlobals(c.GetGlobals())

	result, err := vm.Run(chunk)
	if err != nil {
		console.Println("error: " + err.Error())
		return
	}

	// Print result
	if result != nil {
		console.Println("=> " + result.PrintString())
	}
}

// Error types
type fsError struct{}

func (fsError) Error() string { return "filesystem not available" }

var errNoFS = fsError{}

type eofError struct{}

func (eofError) Error() string { return "EOF" }
