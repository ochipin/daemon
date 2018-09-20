package daemon

import (
	"fmt"
	"io/ioutil"
	"net/http"
	"os"
	"testing"
)

var handleFunc = http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
	fmt.Fprintf(w, "TEST")
})

func Test__FAILED_NOT_IMPLEMENTED(t *testing.T) {
	d, _ := New()
	os.Remove(d.Pidfile)
	d.Daemon(nil)
}

func Test_PIDCHECK(t *testing.T) {
	d, _ := New()
	os.Remove(d.Pidfile)

	// 1. pidfile はあるかチェックする
	fmt.Println("1.", d.Stat())
	if _, err := os.Stat(d.Pidfile); err != nil {
		t.Error("Create Pidfile NG")
	}

	// 2. pidfile がある状態で、再度実行するとすでにあるとエラーになるかチェックする
	fmt.Println("2.", d.Stat())

	// 3. pidfile は存在するが、既に使用されていないプロセス番号の場合は上書きする
	ioutil.WriteFile(d.Pidfile, []byte("99999"), 0600)
	fmt.Println("3.", d.Stat())

	// 4. pidfile ディレクトリの場合は、エラーとする
	os.Remove(d.Pidfile)
	os.MkdirAll(d.Pidfile, 0755)
	d.Stat()

	// 5. pidfile は存在するが、プロセスID以外の情報が記載されている場合
	os.RemoveAll(d.Pidfile)
	ioutil.WriteFile(d.Pidfile, []byte("error test"), 0600)
	d.Stat()

	// 6. pidfile 名が未指定の場合、エラーとする
	var tmp = d.Pidfile
	d.Pidfile = ""
	d.Stat()
	d.Pidfile = tmp
}
