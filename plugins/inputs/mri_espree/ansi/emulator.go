package ansi

import (
	"encoding/base64"
	"errors"
	"io"
	"log"
	"strings"

	ansiterm "github.com/Azure/go-ansiterm"
)

const rows = 24
const cols = 120

type Emulator struct {
	conn       io.ReadWriter
	ansiParser *ansiterm.AnsiParser
	screen     Screen
	pos        Position
	lastState  Screen
}

type Position struct {
	Row int
	Col int
}

func (p *Position) Reset() {
	*p = Position{}
}

type Screen struct {
	characters [rows][cols]byte
}

func (s *Screen) Fill(x byte) {
	for r := range s.characters {
		for c := range s.characters[r] {
			s.characters[r][c] = x
		}
	}
}

func (s *Screen) Erase() {
	s.Fill(' ')
}

func (s *Screen) EraseLineFrom(pos Position) {
	for i := pos.Col; i < len(s.characters[pos.Row]); i++ {
		s.characters[pos.Row][i] = ' '
	}
}

func (s *Screen) Put(chr byte, position Position) {
	if position.Row >= len(s.characters) {
		panic("row overflow")
	}
	if position.Col >= len(s.characters[0]) {
		panic("col overflow")
	}
	s.characters[position.Row][position.Col] = chr
}

func (s *Screen) String() string {
	var rows []string
	for r := range s.characters {
		rows = append(rows, string(s.characters[r][:]))
	}

	return strings.Join(rows, "\n")
}

func NewEmulator(rw io.ReadWriter) *Emulator {
	e := &Emulator{
		conn: rw,
	}
	e.screen.Erase()
	e.ansiParser = ansiterm.CreateParser("Ground", e)
	return e
}

func (e *Emulator) Escape() error {
	return e.Key(27)
}

func (e *Emulator) Enter() error {
	return e.Key(13)
}

func (e *Emulator) Key(b byte) error {
	_, err := e.conn.Write([]byte{b})
	return err
}

func (e *Emulator) Parse(n int) error {
	var buf = make([]byte, n)
	_, err := io.ReadFull(e.conn, buf)
	if err != nil && !errors.Is(err, io.EOF) {
		return err
	}

	_, err = e.ansiParser.Parse(buf[:])
	if err != nil {
		log.Println(base64.StdEncoding.EncodeToString(buf))
		return err
	}

	return nil
}

func (e *Emulator) LastScreen() Screen {
	return e.lastState
}

var _ ansiterm.AnsiEventHandler = (*Emulator)(nil)
