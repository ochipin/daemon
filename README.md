デーモンライブラリ
===
デーモン化ライブラリです。勉強のため、作成しました。

参考URL: https://qiita.com/hironobu_s/items/77d99436457ef57889d6

自分自身をデーモンにする
---
```go
package main

import (
    "fmt"
    "net/http"
    "os"

    "github.com/ochipin/daemon"
)

func main() {
    // デーモン構造体を取得する
    d, err := daemon.New()
    if err != nil {
        panic(err)
    }

    // デーモン化した際に実行する関数を登録する
    d.Exec = HelloWorld
    // デーモン開始
    if err := d.Daemon(os.Args); err != nil {
        os.Exit(2)
    }
}

// デーモン化した際に実行される関数
func HelloWorld() error {
    http.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
        fmt.Fprintf(w, "Hello World")
    })
    return http.ListenAndServe(":8080", nil)
}
```

他のコマンドをデーモンにする
---

#### http.go
```go
package main

import (
    "fmt"
    "net/http"
)

func main() {
    http.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
        fmt.Fprintf(w, "Hello World")
    })

    fmt.Println("Start the web server http://localhost:8080")
    http.ListenAndServe(":8080", nil)
}

// [user@localhost ~]$ go build http.go
// [user@localhost ~]$ ls
// http http.go
```

#### main.go
```go
package main

import (
    "fmt"
    "os"
    "os/exec"
    "sync"
    "syscall"

    "github.com/ochipin/daemon"
)

func main() {
    // デーモン構造体を取得する
    d, err := daemon.New()
    if err != nil {
        panic(err)
    }

    // デーモン化した際に実行する関数を登録する
    d.Exec = HelloWorld
    // デーモン開始
    if err := d.Daemon(os.Args); err != nil {
        os.Exit(2)
    }
}

// ログ構造体
type Log struct {
    mu sync.Mutex
}

// ログ出力関数
func (l *Log) Write(b []byte) (int, error) {
    l.mu.Lock()
    defer l.mu.Unlock()

    // ログ出力処理。。。
    // 今回は、printf での出力のみとする
    fmt.Printf(string(b))

    return len(b), nil
}

// デーモン化した際に実行される関数
func HelloWorld() error {
    // デーモン化したいコマンドを起動する
    cmd := exec.Command("./http")
    cmd.Stdout = &Log{}
    cmd.Stderr = &Log{}
    cmd.Env = os.Environ()
    cmd.SysProcAttr = &syscall.SysProcAttr{
        // 親がKILLされたら、子もKILLする
        Pdeathsig: syscall.SIGKILL | syscall.SIGTERM,
    }
    // 必ず cmd.Run 関数を使用すること
    if err := cmd.Run(); err != nil {
        return err
    }
    return nil
}

// [user@localhost ~]$ go build main.go
// [user@localhost ~]$ ls
// http http.go main main.go
// [user@localhost ~]$ ./main
// Start the web server http://localhost:8080
// [user@localhost ~]$
```

New()で取得したDaemon構造体を変更することで動作を変えることができます。構造体は、次の定義です。

```go
// Daemon : デーモン起動に必要な情報を取り扱う構造体
type Daemon struct {
	StartWait  int          // デーモン化するコマンドの起動待ち時間
	ErrorWait  int          // デーモン化したコマンドのエラー発生待ち時間
	WorkingDir string       // ワーキングディレクトリ
	Appname    string       // アプリケーション名
	Envname    string       // デーモンの起動状態を管理する環境変数名
	Cmdpath    string       // 起動するコマンド名
	Pidfile    string       // PIDファイルのパス
	Stdout     io.Writer    // 標準出力IO(デフォルトはos.Stdout)
	Stderr     io.Writer    // 標準エラー出力IO(デフォルトはos.Stderr)
	Writer     io.Writer    // エラーが発生した際に出力するIO(デフォルトはsyslog)
	Exec       func() error // デーモン起動時にコールされる
}
```