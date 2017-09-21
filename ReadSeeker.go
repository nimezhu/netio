package netio

import (
	"bytes"
	"encoding/binary"
	"errors"
	"fmt"
	"io"
	"io/ioutil"
	"math"
	"net/http"
	"os"
	"regexp"
	"strconv"
	"strings"
)

const (
	defaultBufferSize = 65536 * 16
)

/*
HttpReadSeeker:
	implement io.ReadSeeker inferface for http server which support Range request header
*/
type MutexReadSeeker interface {
	io.ReadSeeker
	Lock()
	Unlock()
}
type HttpReadSeeker struct {
	Url          string
	position     int64
	bufferReader *bytes.Reader
	bufferSize   int
	bufferOffset int64
	size         int64
	bufferMap    map[int64][]byte
}

func Size(uri string) (int64, error) {
	http, _ := regexp.Compile("^http://")
	https, _ := regexp.Compile("^https://")
	switch {
	case http.MatchString(uri) || https.MatchString(uri):
		f, err := NewHttpReadSeeker(uri)
		defer f.Close()
		if err != nil {
			return int64(-1), err
		}
		return f.Size(), nil
	default:
		f, err := os.Open(uri)
		defer f.Close()
		if err != nil {
			return int64(-1), err
		}
		fs, err := f.Stat()
		if err != nil {
			return int64(-1), err
		}
		return fs.Size(), nil
	}
	return int64(-1), io.ErrNoProgress
}

func NewReadSeeker(uri string) (io.ReadSeeker, error) {
	http, _ := regexp.Compile("^http://")
	https, _ := regexp.Compile("^https://")
	switch {
	case http.MatchString(uri) || https.MatchString(uri):
		return NewHttpReadSeeker(uri)
	default:
		return os.Open(uri)
	}
	return nil, io.ErrNoProgress
}
func NewHttpReadSeeker(url string) (*HttpReadSeeker, error) {
	f := HttpReadSeeker{Url: url, bufferMap: make(map[int64][]byte)}
	err := f.Open()
	if err != nil {
		return nil, err
	}
	return &f, nil
}
func (s *HttpReadSeeker) Read(n []byte) (int, error) {
	nl := len(n)
	if nl > s.bufferSize {
		err := s.readHttp(n)
		if err != nil {
			return nl, err
		}
	}
	l, err := s.bufferReader.Read(n)
	s.position += int64(l)
	if s.position == s.size {
		return l, err
	}
	if nl == l {
		return l, nil
	} else {
		err2 := s.readNextBuffer()
		if err2 != nil && err2.Error() == "EOF" {
			return l, err
		} else {
			l1, err := s.Read(n[l:])
			return l + l1, err
		}

	}
	return l, err
}
func (s *HttpReadSeeker) Seek(pos int64, whence int) (int64, error) {
	var newpos int64
	if whence == 0 {
		newpos = pos

	} else if whence == 1 {
		newpos = s.position + pos
	} else if whence == 2 {
		newpos = s.size - pos
	} else {
		newpos = pos
	}
	if newpos > s.size || newpos < 0 {
		return s.position, errors.New("wrong position")
	}
	s.position = newpos
	if newpos > s.bufferOffset && newpos < s.bufferOffset+int64(s.bufferSize) {
		s.bufferReader.Seek(newpos-s.bufferOffset, 0)
	} else {
		s.readNextBuffer()
	}
	return s.position, nil
}

func (s *HttpReadSeeker) readHeader() (http.Header, error) {
	client := &http.Client{}
	req, err := http.NewRequest("GET", s.Url, nil)
	if err != nil {
		return nil, err
	}
	rangeHeader := "bytes=0-1"
	req.Header.Add("Range", rangeHeader)
	resp, err := client.Do(req)
	defer resp.Body.Close()
	if err != nil {
		return nil, err
	}

	return resp.Header, err
}
func (s *HttpReadSeeker) readHttp(n []byte) error { //TODO: reading buffer first?
	client := &http.Client{}
	req, err := http.NewRequest("GET", s.Url, nil)
	if err != nil {
		return err
	}
	if s.position >= s.size {
		s.bufferReader = nil
		return errors.New("EOF")
	}
	length := len(n)
	endv := s.position + int64(length)
	if endv > s.size {
		endv = s.size //+1 ? TO CHECK
	}
	start := fmt.Sprintf("%d", s.position)
	end := fmt.Sprintf("%d", (endv - 1))
	range_header := "bytes=" + start + "-" + end
	//log.Println("read range", range_header) //TODO???
	req.Header.Add("Range", range_header)
	resp, err := client.Do(req) // to fix if query out of size
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	n, err = ioutil.ReadAll(resp.Body)
	if err != nil {
		return err
	}
	s.position = endv - 1
	s.readNextBuffer()
	return err
}
func (s *HttpReadSeeker) readNextBuffer() error {
	v, ok := s.bufferMap[s.Position()]
	if ok {
		s.bufferReader = bytes.NewReader(v)
		s.bufferOffset = s.Position()
		//fmt.Println("skipping buffer", s.Position())
		return nil
	}
	//fmt.Println("reading buffer", s.Position())
	client := &http.Client{}
	req, err := http.NewRequest("GET", s.Url, nil)
	if err != nil {
		return err
	}
	if s.position >= s.size {
		s.bufferReader = nil
		return errors.New("EOF")
	}
	endv := s.position + int64(s.bufferSize)
	if endv > s.size {
		endv = s.size //+1 ? TO CHECK
	}
	start := fmt.Sprintf("%d", s.position)
	end := fmt.Sprintf("%d", endv)
	range_header := "bytes=" + start + "-" + end
	req.Header.Add("Range", range_header)
	//log.Println("reading next", range_header)
	resp, err := client.Do(req) // to fix if query out of size
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	//buf := make([]byte, s.bufferSize)
	//resp.Body.Read(buf)
	buf, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		return err
	}
	//fmt.Println("error bytes?", len(buf))
	s.bufferOffset = s.Position()
	s.bufferMap[s.Position()] = buf
	s.bufferReader = bytes.NewReader(buf)
	return nil
}
func (s *HttpReadSeeker) Open() error {
	s.position = 0
	s.bufferSize = defaultBufferSize
	header, err := s.readHeader()
	size := int64(0)
	//fmt.Println("url header", header) //debug
	if r := header.Get("Content-Range"); r != "" {
		k := strings.Split(r, "/")
		size, _ = strconv.ParseInt(k[1], 10, 64)
	} else {
		size, _ = strconv.ParseInt(header.Get("Content-Length"), 10, 64)
	}
	s.size = size
	err = s.readNextBuffer()
	if err != nil {
		return err
	}
	return nil
}
func (s *HttpReadSeeker) Close() {
	s.position = 0
	s.bufferSize = defaultBufferSize //TODO Close Buffer
}
func (s *HttpReadSeeker) Clone() *HttpReadSeeker { //Clone a new reader
	reader, _ := NewHttpReadSeeker(s.Url)
	return reader
}
func (s *HttpReadSeeker) ReadInt() (int32, error) {
	return ReadInt(s)
}
func (s *HttpReadSeeker) ReadLong() (int64, error) {
	return ReadLong(s)
}
func (s *HttpReadSeeker) ReadString() (string, error) {
	return ReadString(s)
}
func (s *HttpReadSeeker) BufferSize(b int) {
	s.bufferSize = b
	s.readNextBuffer()
}
func (s *HttpReadSeeker) Position() int64 {
	return s.position
}
func (s *HttpReadSeeker) Size() int64 {
	return s.size
}

func ReadString(buf io.Reader) (string, error) {
	buff := new(bytes.Buffer)
	b := make([]byte, 1)
	var err error
	for _, err = buf.Read(b); b[0] != 0 && err == nil; _, err = buf.Read(b) {
		buff.WriteByte(b[0])
	}
	return buff.String(), err
}
func ReadInt(buf io.Reader) (int32, error) {
	c := make([]byte, 4)
	_, err := buf.Read(c)
	return int32(binary.LittleEndian.Uint32(c)), err
}
func ReadUint(buf io.Reader) (uint32, error) {
	c := make([]byte, 4)
	_, err := buf.Read(c)
	return binary.LittleEndian.Uint32(c), err
}
func ReadLong(buf io.Reader) (int64, error) {
	c := make([]byte, 8)
	_, err := buf.Read(c)
	r := int64(binary.LittleEndian.Uint64(c))
	return r, err
}
func ReadUint64(buf io.Reader) (uint64, error) {
	c := make([]byte, 8)
	_, err := buf.Read(c)
	r := binary.LittleEndian.Uint64(c)
	return r, err
}
func ReadFloat64(buf io.Reader) (float64, error) {
	c := make([]byte, 8)
	_, err := buf.Read(c)
	r := math.Float64frombits(binary.LittleEndian.Uint64(c))
	return r, err
}
func ReadFloat32(buf io.Reader) (float32, error) {
	c := make([]byte, 4)
	l, err := buf.Read(c)
	if l != 4 {
		c1 := make([]byte, 2)
		buf.Read(c1)
		c[3] = c1[1]
		c[2] = c1[0]
	}
	r := math.Float32frombits(binary.LittleEndian.Uint32(c)) //TODO
	return r, err
}

func ReadShort(buf io.Reader) (uint16, error) {
	c := make([]byte, 2)
	_, err := buf.Read(c)
	r := binary.LittleEndian.Uint16(c)
	return r, err
}
func ReadByte(buf io.Reader) (byte, error) {
	c := make([]byte, 1)
	_, err := buf.Read(c)
	return c[0], err
}
