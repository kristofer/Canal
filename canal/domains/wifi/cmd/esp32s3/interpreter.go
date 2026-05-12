//go:build tinygo && esp32s3

package main

import (
	"strings"

	"github.com/kristofer/picoceci/pkg/bytecode"
	"github.com/kristofer/picoceci/pkg/eval"
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
		return readModuleFromFS(path)
	})
	module.RegisterBuiltins(resolver)
	loader := module.NewLoader(resolver)

	// Create persistent VM for this REPL session to maintain globals and closures
	// Route Transcript output to TCP console so remote user sees Transcript println: output
	vm := bytecode.NewVMWithSinks(eval.GlobalSinks{
		TranscriptWriter: console,
	})
	installCanalGlobals(vm)

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
					evalREPLSource(console, loader, vm, src)
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

		evalREPLSource(console, loader, vm, line)
	}
}

func evalREPLSource(console consoleIO, loader *module.Loader, vm *bytecode.VM, src string) {
	// Parse
	l := lexer.NewString(src)
	p := parser.New(l)
	prog, err := p.ParseProgram()
	if err != nil {
		console.Println("parse: " + err.Error())
		return
	}

	// Compile with fresh compiler (no seeding required)
	c := bytecode.NewCompilerWithLoader(loader)
	chunk, err := c.Compile(prog.Statements)
	if err != nil {
		console.Println("compile: " + err.Error())
		return
	}

	// Merge blocks from this compilation into VM and adjust bytecode indices
	if err := vm.AddBlocksAndAdjustChunk(chunk, c.GetBlocks()); err != nil {
		console.Println("error: " + err.Error())
		return
	}

	// Keep existing globals and add new ones from this compilation
	vm.AddGlobals(c.GetGlobals())

	// Run the chunk in the persistent VM
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

type eofError struct{}

func (eofError) Error() string { return "EOF" }
