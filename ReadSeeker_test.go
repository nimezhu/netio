package netio

import "testing"

const BIGWIG_MAGIC = 0x888FFC26
const BIGBED_MAGIC = 0x8789F2EB

func TestReadSeeker(t *testing.T) {
	f, _ := NewReadSeeker("https://www.encodeproject.org/files/ENCFF609KNT/@@download/ENCFF609KNT.bigWig")
	v, _ := ReadUint(f)
	t.Log(v)
}
