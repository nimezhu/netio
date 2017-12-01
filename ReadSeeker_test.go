package netio

import (
	"io/ioutil"
	"log"
	"os"
	"testing"
)

const BIGWIG_MAGIC = 0x888FFC26
const BIGBED_MAGIC = 0x8789F2EB

func TestReadSeeker(t *testing.T) {
	f, _ := NewReadSeeker("https://www.encodeproject.org/files/ENCFF609KNT/@@download/ENCFF609KNT.bigWig")
	v, _ := ReadUint(f)
	a, _ := Size("https://www.encodeproject.org/files/ENCFF609KNT/@@download/ENCFF609KNT.bigWig")
	t.Log(v)
	t.Log(a)
}
func TestWrite(t *testing.T) {
	f, _ := ioutil.TempFile("./", "test")
	defer os.Remove(f.Name())
	WriteString(f, "hello,world!")
	Write(f, int32(321))
	Write(f, int64(6464))
	f.Seek(0, 0)
	a, _ := ReadString(f)
	b, _ := ReadInt(f)
	c, _ := ReadLong(f)
	log.Println(a)
	log.Println(b)
	log.Println(c)
}
