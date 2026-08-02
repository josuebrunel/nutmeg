// Command ollama-test sends a sample player-commentary prompt to a local
// Ollama server using the same client and prompt template the app itself
// uses (internal/llm and internal/service.CommentaryService.BuildPrompt),
// so it's a quick way to try out a model before wiring it up for real.
package main

import (
	"context"
	"flag"
	"fmt"
	"os"
	"time"

	"nutmeg/internal/llm"
	"nutmeg/internal/repository"
	"nutmeg/internal/service"
)

func main() {
	url := flag.String("url", "http://localhost:11434", "Ollama base URL")
	model := flag.String("model", "gemma3:270m", "Ollama model name")
	flag.Parse()

	stats := repository.PlayerStats{MatchesPlayed: 5, Wins: 2, Draws: 1, Losses: 2, Goals: 3, Assists: 1}
	history := []repository.PlayerMatchResult{
		{GoalsScored: 0},
		{GoalsScored: 0},
		{GoalsScored: 0},
	}
	prompt := (&service.CommentaryService{}).BuildPrompt("Chris", stats, history, true, false)

	fmt.Println("--- prompt ---")
	fmt.Println(prompt)

	client := llm.NewClient(*url, *model, 60*time.Second)
	fmt.Printf("\n--- response (model=%s, url=%s) ---\n", *model, *url)

	response, err := client.Generate(context.Background(), prompt)
	if err != nil {
		fmt.Fprintln(os.Stderr, "error:", err)
		os.Exit(1)
	}
	fmt.Println(response)
}
