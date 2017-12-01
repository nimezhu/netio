package netio

import (
	"encoding/binary"
	"io"
)

func Write(w io.Writer, n interface{}) {
	binary.Write(w, binary.LittleEndian, n)
}
func WriteString(w io.Writer, s string) {
	w.Write([]byte(s))
	w.Write([]byte{'\x00'})
}
