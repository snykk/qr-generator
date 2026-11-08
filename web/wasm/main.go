// Copyright 2026 Najib Fikri
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0

//go:build js && wasm

// Command wasm is the WebAssembly entry point of the demo page. It exposes
// one function, qrgenCall(method, request, ...blobs), to the JavaScript that
// loads it (a Web Worker in web/site/worker.js) and hands every call to the
// bridge package. Build it with web/build.sh.
package main

import (
	"syscall/js"

	"github.com/snykk/qr-generator/web/internal/bridge"
)

func main() {
	js.Global().Set("qrgenCall", js.FuncOf(func(_ js.Value, args []js.Value) any {
		if len(args) < 2 {
			return `{"error":"qrgenCall needs a method and a JSON request"}`
		}
		var blobs [][]byte
		for _, a := range args[2:] {
			buf := make([]byte, a.Get("length").Int())
			js.CopyBytesToGo(buf, a)
			blobs = append(blobs, buf)
		}
		return string(bridge.Call(args[0].String(), []byte(args[1].String()), blobs))
	}))
	select {} // keep the Go runtime alive for later calls
}
