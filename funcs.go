package netio

import (
	"compress/gzip"
	"io"
	"io/ioutil"
	"os"
	"path"
)

/*ReadAll : read http url/ file / automatically unzip gzip file
 */
func ReadAll(fn string) ([]byte, error) {
	if fn == "STDIN" || fn == "stdin" {
		return ioutil.ReadAll(os.Stdin)
	}
	a, err := NewReadSeeker(fn)
	var gz io.Reader
	if err != nil {
		return []byte(""), err
	} else {
		ext := path.Ext(fn)
		if ext == ".gz" {
			gz, _ = gzip.NewReader(a)
		} else {
			gz = a
		}
	}
	return ioutil.ReadAll(gz)
}
