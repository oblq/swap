package main

import (
	"encoding/json"
	"fmt"

	"github.com/oblq/swap/example/app"
)

func main() {
	tb, _ := json.MarshalIndent(app.ToolBox, "", "  ")
	fmt.Printf("app.Shared: %s", string(tb))
}
