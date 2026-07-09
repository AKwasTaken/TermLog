package main

import (
	"regexp"
	"time"
)

const stateFileName = ".termlog_state.json"

var ansiRegex = regexp.MustCompile(`\x1b\[[0-9;]*[a-zA-Z]`)

type LogMode string

const (
	ModeBelow LogMode = "below"
	ModeAbove LogMode = "above"
	ModeLive  LogMode = "live"
	ModeOff   LogMode = "off"
)

type TerminalSession struct {
	Mode          LogMode   `json:"mode"`
	IsOnline      bool      `json:"is_online"`
	OutputFile    string    `json:"output_file"`
	PID           int       `json:"pid"`
	TerminalType  string    `json:"terminal_type"`
	AnchorMarker  string    `json:"anchor_marker"`
	HistoryCache  string    `json:"history_cache,omitempty"`
	LastLineCount int       `json:"last_line_count"` // Tracks the last processed line position
	LastUpdated   time.Time `json:"last_updated"`
}

type GlobalState struct {
	Sessions map[string]*TerminalSession `json:"sessions"`
}
