package output

import (
	"encoding/json"
	"fmt"
	"os"
)

type Result struct {
	OK      bool   `json:"ok"`
	Command string `json:"command"`
	Result  any    `json:"result,omitempty"`
	Error   string `json:"error,omitempty"`
	Hint    string `json:"hint,omitempty"`
}

func Success(command string, result any) {
	r := Result{OK: true, Command: command, Result: result}
	printJSON(r)
}

func Fail(command string, err error, hint string) {
	r := Result{OK: false, Command: command, Error: err.Error(), Hint: hint}
	printJSON(r)
}

func printJSON(v any) {
	data, err := json.Marshal(v)
	if err != nil {
		fmt.Fprintf(os.Stderr, "kaleidoscope: failed to marshal output: %v\n", err)
		os.Exit(2)
	}
	fmt.Println(string(data))
}
