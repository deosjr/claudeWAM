package main

import "fmt"

// Cell is a 64-bit tagged word. Low 3 bits = tag, upper 61 bits = value.
// This replaces Bowen's Go interface{} with a flat machine word — the key
// to making the WAM efficient. On the real DEC-10 these were 36-bit words
// with 2-3 tag bits, which fit neatly.
type Cell uint64

const (
	TagREF Cell = iota // unbound var (REF pointing to itself) or bound (pointing elsewhere)
	TagSTR             // pointer to a FUN cell on the heap (head of a structure)
	TagATM             // atom: value = atom id
	TagINT             // integer: value = signed int (sign-extended from bit 3)
	TagLIS             // list: value = heap address of [head, tail] pair
	TagFUN             // functor+arity: only appears as first cell of a structure block
)

func (c Cell) Tag() Cell   { return c & 0x7 }
func (c Cell) Val() uint64 { return uint64(c >> 3) }
func (c Cell) Addr() int   { return int(c >> 3) }

func REF(addr int) Cell { return TagREF | Cell(addr)<<3 }
func STR(addr int) Cell { return TagSTR | Cell(addr)<<3 }
func ATM(id int) Cell   { return TagATM | Cell(id)<<3 }
func INT(n int) Cell    { return TagINT | Cell(uint64(int64(n)))<<3 }
func LIS(addr int) Cell { return TagLIS | Cell(addr)<<3 }
func FUN(id int) Cell   { return TagFUN | Cell(id)<<3 }

func (c Cell) IsREF() bool { return c.Tag() == TagREF }
func (c Cell) IsSTR() bool { return c.Tag() == TagSTR }
func (c Cell) IsATM() bool { return c.Tag() == TagATM }
func (c Cell) IsINT() bool { return c.Tag() == TagINT }
func (c Cell) IsLIS() bool { return c.Tag() == TagLIS }
func (c Cell) IsFUN() bool { return c.Tag() == TagFUN }

// An unbound variable is a REF pointing to itself.
func (c Cell) IsUnbound() bool { return c.IsREF() && c.Addr() == -1 }

func (c Cell) String() string {
	switch c.Tag() {
	case TagREF:
		return fmt.Sprintf("REF(%d)", c.Addr())
	case TagSTR:
		return fmt.Sprintf("STR(%d)", c.Addr())
	case TagATM:
		return fmt.Sprintf("ATM(%d)", c.Addr())
	case TagINT:
		return fmt.Sprintf("INT(%d)", int(int64(c.Val())))
	case TagLIS:
		return fmt.Sprintf("LIS(%d)", c.Addr())
	case TagFUN:
		return fmt.Sprintf("FUN(%d)", c.Addr())
	}
	return "?"
}
