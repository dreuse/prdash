package ui

import (
	"time"

	"github.com/charmbracelet/lipgloss"
	"github.com/charmbracelet/x/ansi"
)

func nowMinusDays(n int) time.Time { return time.Now().Add(-time.Duration(n) * 24 * time.Hour) }

func stripANSI(s string) string { return ansi.Strip(s) }

func lipglossWidth(s string) int { return lipgloss.Width(s) }
