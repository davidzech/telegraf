package ansi

import (
	"errors"
	"fmt"
)

// CHA implements ansiterm.AnsiEventHandler.
func (e *Emulator) CHA(n int) error {
	if n-1 < 0 {
		return errors.New("column underflow")
	}
	if n-1 >= cols {
		return errors.New("column overflow")
	}
	e.pos.Col = n - 1
	e.lastState = e.screen
	return nil
}

// CNL implements ansiterm.AnsiEventHandler.
func (e *Emulator) CNL(n int) error {
	e.pos.Col = 0
	if e.pos.Row+n < rows {
		e.pos.Row += n
	} else {
		e.pos.Row = rows - 1
	}

	e.lastState = e.screen
	return nil
}

// CPL implements ansiterm.AnsiEventHandler.
func (e *Emulator) CPL(n int) error {
	e.pos.Col = 0
	if e.pos.Row-n >= 0 {
		e.pos.Row -= n
	} else {
		e.pos.Row = 0
	}
	e.lastState = e.screen
	return nil
}

// CUB implements ansiterm.AnsiEventHandler.
func (e *Emulator) CUB(n int) error {
	e.pos.Col -= n
	if e.pos.Col < 0 {
		e.pos.Col = 0
	}
	return nil
}

// CUD implements ansiterm.AnsiEventHandler.
func (e *Emulator) CUD(n int) error {
	e.pos.Row += n
	if e.pos.Row >= rows {
		e.pos.Row = rows - 1
	}
	e.lastState = e.screen
	return nil
}

// CUF implements ansiterm.AnsiEventHandler.
func (e *Emulator) CUF(n int) error {
	e.pos.Col++
	if e.pos.Col >= cols {
		e.pos.Col = cols - 1
	}

	return nil
}

// CUP implements ansiterm.AnsiEventHandler.
func (e *Emulator) CUP(n int, m int) error {
	e.pos = Position{
		Row: n - 1,
		Col: m - 1,
	}
	if e.pos.Row < 0 {
		e.pos.Row = 0
	}
	if e.pos.Row >= rows {
		e.pos.Row = rows - 1
	}

	if e.pos.Col < 0 {
		e.pos.Col = 0
	}
	if e.pos.Col >= cols {
		e.pos.Col = cols - 1
	}
	if n != 1 && m != 1 {
		e.lastState = e.screen
	}
	return nil
}

// CUU implements ansiterm.AnsiEventHandler.
func (e *Emulator) CUU(n int) error {
	e.pos.Row -= n
	if e.pos.Row < 0 {
		e.pos.Row = 0
	}
	return nil
}

// DA implements ansiterm.AnsiEventHandler.
func (e *Emulator) DA([]string) error {
	return errors.New("da not supported")
}

// DCH implements ansiterm.AnsiEventHandler.
func (e *Emulator) DCH(n int) error {
	return errors.New("dch not supported")
}

// DECCOLM implements ansiterm.AnsiEventHandler.
func (e *Emulator) DECCOLM(bool) error {
	return errors.New("deccolm not supported")
}

// DECOM implements ansiterm.AnsiEventHandler.
func (e *Emulator) DECOM(bool) error {
	return errors.New("decom not supported")
}

// DECSTBM implements ansiterm.AnsiEventHandler.
func (e *Emulator) DECSTBM(int, int) error {
	return errors.New("decstbm not supported")
}

// DECTCEM implements ansiterm.AnsiEventHandler.
func (e *Emulator) DECTCEM(bool) error {
	return errors.New("dectcem not supported")
}

// DL implements ansiterm.AnsiEventHandler.
func (e *Emulator) DL(int) error {
	return errors.New("dl not supported")
}

// ED implements ansiterm.AnsiEventHandler.
func (e *Emulator) ED(n int) error {
	e.lastState = e.screen
	switch n {
	case 0:
		e.screen.EraseLineFrom(e.pos)

	case 2:
		e.screen.Erase()
		// this may not be a thing
		e.pos.Reset()

	case 3:
		e.screen.Erase()

	default:
		return fmt.Errorf("unsupported ED: %v", n)
	}
	return nil
}

// EL implements ansiterm.AnsiEventHandler.
func (e *Emulator) EL(int) error {
	return errors.New("el not supported")
}

// Execute implements ansiterm.AnsiEventHandler.
func (e *Emulator) Execute(b byte) error {
	switch b {
	case '\n':
		if e.pos.Row+1 < rows {
			e.pos.Row++
		}
	case '\r':
		e.pos.Col = 0
	default:
		// return fmt.Errorf("unsupported execute seq: %v", b)
		// log.Printf("unknown execute seq: %v\n", b)
	}
	return nil
}

// Flush implements ansiterm.AnsiEventHandler.
func (e *Emulator) Flush() error {
	return nil
}

// HVP implements ansiterm.AnsiEventHandler.
func (e *Emulator) HVP(int, int) error {
	return errors.New("hvp not supported")
}

// ICH implements ansiterm.AnsiEventHandler.
func (e *Emulator) ICH(int) error {
	return errors.New("ich not supported")
}

// IL implements ansiterm.AnsiEventHandler.
func (e *Emulator) IL(int) error {
	return errors.New("IL not supported")
}

// IND implements ansiterm.AnsiEventHandler.
func (e *Emulator) IND() error {
	return errors.New("ind not supported")
}

// Print implements ansiterm.AnsiEventHandler.
func (e *Emulator) Print(b byte) error {
	if e.pos.Col >= cols {
		println("WARNING: Overprinting line")
		return nil
	}
	if e.pos.Row >= rows {
		println("WARNING: Overprinting rows")
		return nil
	}

	e.screen.Put(b, e.pos)
	e.pos.Col++
	return nil
}

// RI implements ansiterm.AnsiEventHandler.
func (e *Emulator) RI() error {
	return errors.New("ri not supported")
}

// SD implements ansiterm.AnsiEventHandler.
func (e *Emulator) SD(int) error {
	return errors.New("sd not supported")
}

// SGR implements ansiterm.AnsiEventHandler.
func (e *Emulator) SGR(nm []int) error {
	if len(nm) == 1 && nm[0] == 0 {
		// Reset
	}
	return nil
}

// SU implements ansiterm.AnsiEventHandler.
func (e *Emulator) SU(lines int) error {
	return errors.New("su not supported")
}

// VPA implements ansiterm.AnsiEventHandler.
func (e *Emulator) VPA(x int) error {
	return fmt.Errorf("VPA unsupported %v", x)
}
