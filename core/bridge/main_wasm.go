//go:build js && wasm

package main

import (
	"fmt"
	"syscall/js"

	"github.com/jun/gophdrive/core/markdown"
	"github.com/jun/gophdrive/core/sync"
)

func main() {
	renderer := markdown.NewRenderer()

	// format: renderMarkdown(sourceString) -> htmlString
	renderFunc := js.FuncOf(func(this js.Value, args []js.Value) interface{} {
		if len(args) != 1 {
			return "Error: Invalid number of arguments"
		}
		source := args[0].String()

		htmlBytes, err := renderer.Render([]byte(source))
		if err != nil {
			return "Error: " + err.Error()
		}

		return string(htmlBytes)
	})

	// format: checkConflict(localEtag, remoteEtag string) -> bool
	checkConflictFunc := js.FuncOf(func(this js.Value, args []js.Value) interface{} {
		if len(args) != 2 {
			return false
		}
		localEtag := args[0].String()
		remoteEtag := args[1].String()
		return sync.CheckConflict(localEtag, remoteEtag)
	})

	// format: createOfflineChange(noteID, content string) -> object
	createOfflineChangeFunc := js.FuncOf(func(this js.Value, args []js.Value) interface{} {
		if len(args) != 2 {
			return nil
		}
		noteID := args[0].String()
		content := args[1].String()

		change := sync.NewOfflineChange(noteID, content)

		obj := js.Global().Get("Object").New()
		obj.Set("noteId", change.NoteID)
		obj.Set("content", change.Content)
		obj.Set("timestamp", change.Timestamp)

		return obj
	})

	js.Global().Set("renderMarkdown", renderFunc)
	js.Global().Set("checkConflict", checkConflictFunc)
	js.Global().Set("createOfflineChange", createOfflineChangeFunc)

	fmt.Println("GophDrive Core Wasm Initialized")

	// Prevent the function from returning, which would exit the Wasm module
	select {}
}
